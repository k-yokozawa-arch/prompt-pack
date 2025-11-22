package auditzip

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"
)

type Service struct {
	cfg    Config
	queue  *JobQueue
	audit  AuditRecorder
	logger *slog.Logger
}

func NewService(cfg Config, queue *JobQueue, audit AuditRecorder, logger *slog.Logger) Service {
	return Service{cfg: cfg, queue: queue, audit: audit, logger: logger}
}

func (s Service) EnqueueAuditZip(w http.ResponseWriter, r *http.Request) {
	ctx, corrID, tenantID, err := withRequestContext(r)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, ValidationError{Errors: []ValidationErrorItem{{Code: "BAD_REQUEST", Path: "headers", Message: err.Error()}}})
		return
	}
	log := CorrelationLogger(s.logger, corrID, tenantID)

	req, err := decodeRequest(r.Body)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, ValidationError{Errors: []ValidationErrorItem{{Code: "BAD_JSON", Path: "body", Message: err.Error()}}})
		return
	}
	if errs := ValidateRequest(req); len(errs) > 0 {
		writeJSON(w, http.StatusBadRequest, ValidationError{Errors: errs})
		return
	}

	job := s.queue.Enqueue(ctx, tenantID, req)
	if err := s.appendAudit(ctx, tenantID, corrID, "audit.zip.create"); err != nil {
		log.Warn("audit append failed", "error", err)
	}
	writeJSONStatus(w, http.StatusAccepted, job)
}

func (s Service) GetAuditZipJob(w http.ResponseWriter, r *http.Request, jobID string) {
	ctx, corrID, tenantID, err := withRequestContext(r)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, ValidationError{Errors: []ValidationErrorItem{{Code: "BAD_REQUEST", Path: "headers", Message: err.Error()}}})
		return
	}
	log := CorrelationLogger(s.logger, corrID, tenantID)

	job, ok := s.queue.Get(jobID)
	if !ok {
		writeJSON(w, http.StatusNotFound, map[string]string{"code": "NOT_FOUND", "message": "job not found"})
		return
	}
	if err := s.appendAudit(ctx, tenantID, corrID, "audit.zip.get"); err != nil {
		log.Warn("audit append failed", "error", err)
	}
	writeJSON(w, http.StatusOK, job)
}

func decodeRequest(body io.ReadCloser) (AuditZipRequest, error) {
	defer body.Close()
	var req AuditZipRequest
	if err := json.NewDecoder(body).Decode(&req); err != nil {
		return req, fmt.Errorf("invalid JSON: %w", err)
	}
	return req, nil
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	writeJSONStatus(w, status, v)
}

func writeJSONStatus(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func withRequestContext(r *http.Request) (context.Context, string, string, error) {
	corr := r.Header.Get("X-Correlation-Id")
	tenant := r.Header.Get("X-Tenant-Id")
	if corr == "" || tenant == "" {
		return r.Context(), corr, tenant, errors.New("missing X-Correlation-Id or X-Tenant-Id")
	}
	ctx := context.WithValue(r.Context(), "corrId", corr)
	ctx = context.WithValue(ctx, "tenantId", tenant)
	return ctx, corr, tenant, nil
}

func (s Service) appendAudit(ctx context.Context, tenantID, corrID, action string) error {
	if s.audit == nil {
		return nil
	}
	entry := AuditLog{
		AuditID:  newID(),
		CorrID:   corrID,
		TenantID: tenantID,
		Actor:    "system",
		Action:   action,
		Ts:       time.Now().UTC(),
	}
	_, err := HashChain(ctx, s.audit, tenantID, entry)
	return err
}

type MemoryAuditRecorder struct {
	byTenant map[string][]AuditLog
}

func NewMemoryAuditRecorder() *MemoryAuditRecorder {
	return &MemoryAuditRecorder{byTenant: map[string][]AuditLog{}}
}

func (m *MemoryAuditRecorder) Append(_ context.Context, entry AuditLog) error {
	m.byTenant[entry.TenantID] = append(m.byTenant[entry.TenantID], entry)
	return nil
}

func (m *MemoryAuditRecorder) Last(_ context.Context, tenantID string) (AuditLog, error) {
	list := m.byTenant[tenantID]
	if len(list) == 0 {
		return AuditLog{}, fmt.Errorf("empty")
	}
	return list[len(list)-1], nil
}

// PathParamJobID extracts jobId from /audit/jobs/{jobId} paths for the bare mux in cmd.
func PathParamJobID(path string) string {
	parts := strings.Split(strings.Trim(path, "/"), "/")
	if len(parts) == 3 && parts[0] == "audit" && parts[1] == "jobs" {
		return parts[2]
	}
	return ""
}
