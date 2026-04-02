package main

import (
	"context"
	"embed"
	"io/fs"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"

	"github.com/rzfd/metatech/konkon/internal/config"
	"github.com/rzfd/metatech/konkon/internal/httpapi"
	"github.com/rzfd/metatech/konkon/internal/store"
)

//go:embed all:web
var webFS embed.FS

func main() {
	log := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	cfg := config.Load()
	if err := os.MkdirAll(cfg.DataDir, 0o750); err != nil {
		log.Error("mkdir data", "err", err)
		os.Exit(1)
	}
	if err := os.MkdirAll(cfg.UploadDir, 0o750); err != nil {
		log.Error("mkdir uploads", "err", err)
		os.Exit(1)
	}
	dbPath, err := filepath.Abs(cfg.DBPath)
	if err != nil {
		log.Error("abs db", "err", err)
		os.Exit(1)
	}
	uploadPath, err := filepath.Abs(cfg.UploadDir)
	if err != nil {
		log.Error("abs uploads", "err", err)
		os.Exit(1)
	}
	st, err := store.Open(context.Background(), cfg.DBDriver, dbPath, cfg.PostgresDSN)
	if err != nil {
		log.Error("open db", "err", err)
		os.Exit(1)
	}
	defer st.Close()

	mux := http.NewServeMux()
	api := httpapi.New(log, st, uploadPath)
	api.Register(mux)

	sub, err := fs.Sub(webFS, "web")
	if err != nil {
		log.Error("web fs", "err", err)
		os.Exit(1)
	}
	mux.Handle("/", http.FileServer(http.FS(sub)))

	log.Info("listening", "addr", cfg.ListenAddr)
	if err := http.ListenAndServe(cfg.ListenAddr, mux); err != nil {
		log.Error("server", "err", err)
		os.Exit(1)
	}
}
