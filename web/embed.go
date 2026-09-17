package web

import "embed"

// Dist embeds the built frontend (Vite output). Dev placeholder exists
// until the real `web/dist` is built; the server falls back to API-only
// if the subtree is missing.

//go:embed dist/*
var Dist embed.FS
