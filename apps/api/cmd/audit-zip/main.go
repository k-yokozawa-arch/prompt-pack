package main

import (
	"log/slog"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/yourorg/yourapp/apps/api/internal/auditzip"
	"github.com/yourorg/yourapp/apps/api/internal/pint"
	"github.com/joho/godotenv"
)

func main() {
	_ = godotenv.Load(".env")

	cfg := auditzip.LoadConfig()
	storage := auditzip.NewInMemoryStorage()
	queue := auditzip.NewJobQueue(storage, cfg)
	audit := auditzip.NewMemoryAuditRecorder()
	svc := auditzip.NewService(cfg, queue, audit, slog.Default())

	// JP PINT invoice service (shares server for local dev).
	pCfg := pint.LoadConfig()
	pStorage := pint.NewInMemoryStorage()
	pAudit := pint.NewMemoryAuditRecorder()
	pSvc := pint.NewService(pCfg, pStorage, pAudit, slog.Default())

	router := chi.NewRouter()
	router.Use(corsMiddleware(cfg.AllowedOrigins))
	handler := auditzip.HandlerFromMuxWithBaseURL(svc, router, "")

	// Invoice endpoints
	router.Post("/invoices/validate", pSvc.ValidateInvoice)
	router.Post("/invoices", pSvc.IssueInvoice)
	router.Get("/invoices/{id}", func(w http.ResponseWriter, r *http.Request) {
		pSvc.GetInvoice(w, r, chi.URLParam(r, "id"))
	})
	router.Get("/storage/*", func(w http.ResponseWriter, r *http.Request) {
		key := strings.TrimPrefix(r.URL.Path, "/storage/")
		body, ctype, err := pStorage.GetObject(r.Context(), key)
		if err != nil {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", ctype)
		_, _ = w.Write(body)
	})

	addr := ":8080"
	slog.Info("audit-zip api listening", "addr", addr)
	if err := http.ListenAndServe(addr, handler); err != nil {
		slog.Error("server stopped", "error", err)
	}
}

// corsMiddleware allows configured origins for dev (e.g., Next.js on :3000).
func corsMiddleware(allowed []string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			origin := r.Header.Get("Origin")
			if origin != "" && isAllowedOrigin(origin, allowed) {
				w.Header().Set("Access-Control-Allow-Origin", origin)
				w.Header().Set("Vary", "Origin")
				w.Header().Set("Access-Control-Allow-Credentials", "true")
				w.Header().Set("Access-Control-Allow-Headers", "Content-Type, X-Correlation-Id, X-Tenant-Id, Idempotency-Key, Authorization")
				w.Header().Set("Access-Control-Allow-Methods", "GET,POST,OPTIONS")
			}
			if r.Method == http.MethodOptions {
				w.WriteHeader(http.StatusNoContent)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

func isAllowedOrigin(origin string, allowed []string) bool {
	if len(allowed) == 0 {
		return false
	}
	for _, a := range allowed {
		if a == "*" || strings.EqualFold(strings.TrimSpace(a), origin) {
			return true
		}
	}
	return false
}
