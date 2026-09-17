package main

import (
	"flag"
	"fmt"
	"io/fs"
	"log"
	"net/http"
	"os"

	_ "modernc.org/sqlite"

	"github.com/EziosWJ/trade-ledger/internal/handler"
	"github.com/EziosWJ/trade-ledger/internal/service"
	"github.com/EziosWJ/trade-ledger/internal/store"
	"github.com/EziosWJ/trade-ledger/migrations"
	webassets "github.com/EziosWJ/trade-ledger/web"
)

func main() {
	var (
		addr        = flag.String("addr", ":8080", "listen address")
		dsn         = flag.String("db", "trade-ledger.db", "sqlite DSN (file path)")
		migrateOnly = flag.Bool("migrate-only", false, "apply migrations then exit")
	)
	flag.Parse()

	db, err := store.Open(*dsn)
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()
	if err := store.Migrate(db, migrations.FS); err != nil {
		log.Fatal(err)
	}
	if *migrateOnly {
		fmt.Println("migrations applied")
		return
	}

	svc := service.New(db)

	var webFS http.FileSystem
	if sub, err := fs.Sub(webassets.Dist, "dist"); err == nil {
		if _, err := fs.Stat(sub, "index.html"); err == nil {
			webFS = http.FS(sub)
		}
	}
	if webFS != nil {
		fmt.Fprintln(os.Stdout, "web assets embedded")
	} else {
		fmt.Fprintln(os.Stderr, "web/dist not built, serving API only")
	}

	fmt.Printf("trade-ledger listening on %s (db=%s)\n", *addr, *dsn)
	log.Fatal(http.ListenAndServe(*addr, handler.New(svc, webFS)))
}
