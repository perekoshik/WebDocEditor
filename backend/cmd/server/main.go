package main

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"os"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"

	"legaledit/internal/audit"
	"legaledit/internal/documents"
	"legaledit/internal/files"
	"legaledit/internal/onlyoffice"
	"legaledit/internal/storage"
)

type config struct {
	DatabaseURL        string
	FilesDir           string
	PublicAPIURL       string
	InternalAPIURL     string
	OnlyOfficeInternal string
	OnlyOfficePublic   string
	ServerAddr         string
	WebDir             string
}

func loadConfig() config {
	return config{
		DatabaseURL:        env("DATABASE_URL", "postgres://localhost:5432/legaledit?sslmode=disable"),
		FilesDir:           env("FILES_DIR", "./data/uploads"),
		PublicAPIURL:       env("PUBLIC_API_URL", "http://localhost:8080"),
		InternalAPIURL:     env("INTERNAL_API_URL", "http://host.docker.internal:8080"),
		OnlyOfficeInternal: env("ONLYOFFICE_INTERNAL_URL", "http://localhost:8081"),
		OnlyOfficePublic:   env("ONLYOFFICE_PUBLIC_URL", "http://localhost:8081"),
		ServerAddr:         env("SERVER_ADDR", ":8080"),
		WebDir:             env("WEB_DIR", "./web"),
	}
}

func env(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func main() {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	slog.SetDefault(logger)

	cfg := loadConfig()

	if err := os.MkdirAll(cfg.FilesDir, 0o755); err != nil {
		logger.Error("create files dir", "err", err)
		os.Exit(1)
	}

	ctx := context.Background()
	pool, err := storage.NewPool(ctx, cfg.DatabaseURL)
	if err != nil {
		logger.Error("postgres connect", "err", err)
		os.Exit(1)
	}
	defer pool.Close()

	store := &files.Storage{Dir: cfg.FilesDir}
	repo := documents.NewRepository(pool)

	httpClient := &http.Client{Timeout: 60 * time.Second}
	ooClient := onlyoffice.NewClient(cfg.OnlyOfficeInternal, httpClient)
	builder := &onlyoffice.ConfigBuilder{
		InternalAPIURL: cfg.InternalAPIURL,
		PublicAPIURL:   cfg.PublicAPIURL,
	}

	auditLog := audit.New(pool, logger)
	handler := documents.NewHandler(repo, store, builder, ooClient, httpClient, auditLog, logger)

	r := chi.NewRouter()
	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)

	r.Route("/api", func(r chi.Router) {
		r.Get("/config", func(w http.ResponseWriter, req *http.Request) {
			writeJSON(w, http.StatusOK, map[string]string{
				"onlyofficeUrl":  cfg.OnlyOfficePublic,
				"internalApiUrl": cfg.InternalAPIURL,
			})
		})
		r.Post("/documents", handler.Upload)
		r.Get("/documents", handler.List)
		r.Get("/documents/{id}", handler.Get)
		r.Patch("/documents/{id}", handler.Rename)
		r.Delete("/documents/{id}", handler.Delete)
		r.Get("/documents/{id}/file", handler.File)
		r.Post("/documents/{id}/callback", handler.Callback)
		r.Post("/documents/{id}/instantiate", handler.Instantiate)
		r.Get("/documents/{id}/versions", handler.Versions)
		r.Get("/documents/{id}/versions/{version}/file", handler.VersionFile)
		r.Get("/documents/{id}/export", handler.Export)
	})

	webFS := http.FileServer(http.Dir(cfg.WebDir))
	r.Handle("/static/*", http.StripPrefix("/static", webFS))
	r.Handle("/*", webFS)

	logger.Info("listening", "addr", cfg.ServerAddr, "web", cfg.WebDir)
	if err := http.ListenAndServe(cfg.ServerAddr, r); err != nil {
		logger.Error("server", "err", err)
		os.Exit(1)
	}
}

func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}
