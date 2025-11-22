package main

import (
	"log/slog"
	"net/http"

	"github.com/yourorg/yourapp/apps/api/internal/auditzip"
)

func main() {
	cfg := auditzip.LoadConfig()
	storage := auditzip.NewInMemoryStorage()
	queue := auditzip.NewJobQueue(storage, cfg)
	audit := auditzip.NewMemoryAuditRecorder()
	svc := auditzip.NewService(cfg, queue, audit, slog.Default())

	mux := http.NewServeMux()
	mux.HandleFunc("/audit/zip", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		svc.EnqueueAuditZip(w, r)
	})
	mux.HandleFunc("/audit/jobs/", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		jobID := auditzip.PathParamJobID(r.URL.Path)
		if jobID == "" {
			http.NotFound(w, r)
			return
		}
		svc.GetAuditZipJob(w, r, jobID)
	})

	addr := ":8080"
	slog.Info("audit-zip api listening", "addr", addr)
	err := http.ListenAndServe(addr, mux)
	if err != nil {
		slog.Error("server stopped", "error", err)
	}
}
