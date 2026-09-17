package handler

import (
	"crypto/subtle"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"os"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"

	"github.com/EziosWJ/trade-ledger/internal/service"
)

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	if err := json.NewEncoder(w).Encode(v); err != nil {
		log.Printf("write JSON response: %v", err)
	}
}

func writeErr(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, service.ErrBadRequest),
		errors.Is(err, service.ErrOverpay):
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
	case errors.Is(err, service.ErrNotFound):
		writeJSON(w, http.StatusNotFound, map[string]string{"error": err.Error()})
	default:
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
	}
}

func auth(password string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if password == "" {
				next.ServeHTTP(w, r)
				return
			}
			h := strings.Fields(r.Header.Get("Authorization"))
			bearer := ""
			if len(h) == 2 && strings.EqualFold(h[0], "Bearer") {
				bearer = h[1]
			}
			xToken := r.Header.Get("X-API-Token")
			bearerOK := subtle.ConstantTimeCompare([]byte(bearer), []byte(password)) == 1
			xTokenOK := subtle.ConstantTimeCompare([]byte(xToken), []byte(password)) == 1
			if !bearerOK && !xTokenOK {
				writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

func New(svc *service.Service, webDist http.FileSystem) http.Handler {
	r := chi.NewRouter()
	r.Use(middleware.Recoverer, middleware.Logger)

	pw := os.Getenv("TRADE_LEDGER_PASSWORD")

	r.Get("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, 200, map[string]string{"status": "ok"})
	})

	r.Route("/api", func(r chi.Router) {
		r.Use(auth(pw))

		r.Get("/overview", func(w http.ResponseWriter, _ *http.Request) {
			o, err := svc.Overview()
			if err != nil {
				writeErr(w, err)
				return
			}
			writeJSON(w, 200, o)
		})

		r.Post("/counterparties", func(w http.ResponseWriter, r *http.Request) {
			var req struct {
				Name  string   `json:"name"`
				Phone string   `json:"phone"`
				Note  string   `json:"note"`
				Tags  []string `json:"tags"`
			}
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				writeErr(w, service.ErrBadRequest)
				return
			}
			c, err := svc.CreateCounterparty(req.Name, req.Phone, req.Note, req.Tags)
			if err != nil {
				writeErr(w, err)
				return
			}
			writeJSON(w, 201, c)
		})

		r.Get("/counterparties", func(w http.ResponseWriter, _ *http.Request) {
			list, err := svc.ListCounterparties()
			if err != nil {
				writeErr(w, err)
				return
			}
			writeJSON(w, 200, list)
		})

		r.Get("/counterparties/{id}/timeline", func(w http.ResponseWriter, r *http.Request) {
			tl, err := svc.Timeline(chi.URLParam(r, "id"))
			if err != nil {
				writeErr(w, err)
				return
			}
			if tl == nil {
				tl = []service.TimelineEntry{}
			}
			writeJSON(w, 200, tl)
		})

		r.Post("/offsets", func(w http.ResponseWriter, r *http.Request) {
			var req struct {
				ReceivableTradeID string `json:"receivable_trade_id"`
				PayableTradeID    string `json:"payable_trade_id"`
				AmountCents       int64  `json:"amount_cents"`
				Note              string `json:"note"`
			}
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				writeErr(w, service.ErrBadRequest)
				return
			}
			offset, err := svc.AddOffset(req.ReceivableTradeID, req.PayableTradeID, req.AmountCents, req.Note)
			if err != nil {
				writeErr(w, err)
				return
			}
			writeJSON(w, http.StatusCreated, offset)
		})

		r.Post("/trades", func(w http.ResponseWriter, r *http.Request) {
			var req struct {
				CounterpartyID string              `json:"counterparty_id"`
				Type           string              `json:"type"`
				Direction      string              `json:"direction"`
				TotalCents     int64               `json:"total_cents"`
				Note           string              `json:"note"`
				Items          []service.TradeItem `json:"items"`
			}
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				writeErr(w, service.ErrBadRequest)
				return
			}
			t, err := svc.CreateTrade(req.CounterpartyID, req.Type, req.Direction, req.TotalCents, req.Note, req.Items)
			if err != nil {
				writeErr(w, err)
				return
			}
			writeJSON(w, 201, t)
		})

		r.Get("/trades", func(w http.ResponseWriter, r *http.Request) {
			list, err := svc.ListTrades(r.URL.Query().Get("counterparty_id"))
			if err != nil {
				writeErr(w, err)
				return
			}
			writeJSON(w, 200, list)
		})

		r.Post("/trades/{id}/payments", func(w http.ResponseWriter, r *http.Request) {
			var req struct {
				AmountCents int64  `json:"amount_cents"`
				Note        string `json:"note"`
			}
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				writeErr(w, service.ErrBadRequest)
				return
			}
			paid, err := svc.AddPayment(chi.URLParam(r, "id"), req.AmountCents, req.Note)
			if err != nil {
				writeErr(w, err)
				return
			}
			writeJSON(w, 201, map[string]any{"paid_cents": paid})
		})
	})

	if webDist != nil {
		fileServer := http.FileServer(webDist)
		r.Handle("/*", http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			if req.URL.Path == "/api" || strings.HasPrefix(req.URL.Path, "/api/") {
				http.NotFound(w, req)
				return
			}
			path := strings.TrimPrefix(req.URL.Path, "/")
			if !strings.Contains(path, ".") {
				req.URL.Path = "/"
			}
			fileServer.ServeHTTP(w, req)
		}))
	}
	return r
}
