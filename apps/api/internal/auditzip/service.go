package auditzip

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"time"

	openapi_types "github.com/oapi-codegen/runtime/types"
)

type Service struct {
	cfg     Config
	queue   *JobQueue
	audit   AuditRecorder
	logger  *slog.Logger
	limiter *RateLimiter
}

func NewService(cfg Config, queue *JobQueue, audit AuditRecorder, logger *slog.Logger) Service {
	if logger == nil {
		logger = slog.Default()
	}
	return Service{
		cfg:     cfg,
		queue:   queue,
		audit:   audit,
		logger:  logger,
		limiter: NewRateLimiter(cfg.RateLimitPerMinute, time.Minute),
	}
}

func (s Service) EnqueueAuditZip(w http.ResponseWriter, r *http.Request, params EnqueueAuditZipParams) {
	corrID := params.XCorrelationId.String()
	tenantID := string(params.XTenantId)
	idempotencyKey := params.IdempotencyKey.String()
	log := CorrelationLogger(s.logger, corrID, tenantID)

	if ok, retryAfter := s.limiter.Allow(tenantID); !ok {
		body := RateLimitError{Code: "RATE_LIMITED", Message: "too many requests", CorrId: corrID, Retryable: true, RetryAfterSeconds: toRetrySeconds(retryAfter)}
		writeJSON(w, http.StatusTooManyRequests, corrID, body, map[string]string{"Retry-After": formatRetryAfter(retryAfter)})
		return
	}

	req, err := decodeRequest(r.Body)
	if err != nil {
		body := ValidationError{
			Code:      "BAD_JSON",
			Message:   "invalid JSON",
			CorrId:    corrID,
			Retryable: false,
			Errors:    []ValidationErrorItem{{Code: "BAD_JSON", Path: "body", Message: err.Error()}},
		}
		writeJSON(w, http.StatusBadRequest, corrID, body, nil)
		return
	}
	errs, hint := ValidateRequest(req, s.cfg)
	if len(errs) > 0 {
		body := ValidationError{
			Code:      "VALIDATION_ERROR",
			Message:   "request validation failed",
			CorrId:    corrID,
			Retryable: false,
			Errors:    errs,
		}
		writeJSON(w, http.StatusBadRequest, corrID, body, nil)
		return
	}
	if hint != nil {
		body := RequestTooLargeError{
			Code:      "AUDIT-REQ-413",
			Message:   "result exceeds threshold; split by hint",
			CorrId:    corrID,
			Retryable: false,
			SplitHint: *hint,
		}
		writeJSON(w, http.StatusRequestEntityTooLarge, corrID, body, nil)
		return
	}

	criteriaHash := computeCriteriaHash(tenantID, req)
	job, err := s.queue.Enqueue(context.Background(), tenantID, idempotencyKey, criteriaHash, req)
	if err != nil {
		switch e := err.(type) {
		case ConflictErr:
			body := ConflictError{
				Code:           "CONFLICT",
				Message:        conflictMessage(e),
				CorrId:         corrID,
				Retryable:      false,
				ConflictReason: e.Reason,
			}
			writeJSON(w, http.StatusConflict, corrID, body, nil)
			return
		case RateLimitErr:
			body := RateLimitError{
				Code:              "RATE_LIMITED",
				Message:           "queue is full",
				CorrId:            corrID,
				Retryable:         true,
				RetryAfterSeconds: toRetrySeconds(e.RetryAfter),
			}
			writeJSON(w, http.StatusTooManyRequests, corrID, body, map[string]string{"Retry-After": formatRetryAfter(e.RetryAfter)})
			return
		default:
			s.writeInternalError(w, corrID, err)
			return
		}
	}

	_ = s.appendAudit(context.Background(), tenantID, corrID, "audit.zip.create", criteriaHash)

	location := fmt.Sprintf("/audit/jobs/%s", job.JobId)
	writeJSON(w, http.StatusAccepted, corrID, s.decorateJob(job, corrID), map[string]string{"Location": location})
	log.Info("audit zip job enqueued", "jobId", job.JobId, "criteriaHash", criteriaHash)
}

func (s Service) GetAuditZipJob(w http.ResponseWriter, r *http.Request, jobID openapi_types.UUID, params GetAuditZipJobParams) {
	corrID := params.XCorrelationId.String()
	tenantID := string(params.XTenantId)
	log := CorrelationLogger(s.logger, corrID, tenantID)

	job, jobTenant, ok := s.queue.Get(jobID.String())
	if !ok || jobTenant != tenantID {
		body := NotFoundError{Code: "NOT_FOUND", Message: "job not found", CorrId: corrID, Retryable: false}
		writeJSON(w, http.StatusNotFound, corrID, body, nil)
		return
	}

	if params.Cancel != nil && *params.Cancel {
		updated, err := s.queue.Cancel(tenantID, jobID.String())
		if err != nil {
			switch e := err.(type) {
			case ConflictErr:
				body := ConflictError{
					Code:           "CONFLICT",
					Message:        "job cannot be canceled in current state",
					CorrId:         corrID,
					Retryable:      false,
					ConflictReason: e.Reason,
				}
				writeJSON(w, http.StatusConflict, corrID, body, nil)
				return
			default:
				s.writeInternalError(w, corrID, err)
				return
			}
		}
		job = updated
		_ = s.appendAudit(context.Background(), tenantID, corrID, "audit.zip.cancel", deref(job.CriteriaHash))
	} else {
		_ = s.appendAudit(context.Background(), tenantID, corrID, "audit.zip.get", deref(job.CriteriaHash))
	}

	writeJSON(w, http.StatusOK, corrID, s.decorateJob(job, corrID), nil)
	log.Info("audit zip job fetched", "jobId", job.JobId, "status", job.Status)
}

func (s Service) writeInternalError(w http.ResponseWriter, corrID string, err error) {
	body := InternalError{Code: "INTERNAL_ERROR", Message: err.Error(), CorrId: corrID, Retryable: true}
	writeJSON(w, http.StatusInternalServerError, corrID, body, nil)
}

func decodeRequest(body io.ReadCloser) (AuditZipRequest, error) {
	defer body.Close()
	var req AuditZipRequest
	if err := json.NewDecoder(body).Decode(&req); err != nil {
		return req, err
	}
	return req, nil
}

func writeJSON(w http.ResponseWriter, status int, corrID string, v any, extra map[string]string) {
	w.Header().Set("Content-Type", "application/json")
	if corrID != "" {
		w.Header().Set("X-Correlation-Id", corrID)
	}
	for k, val := range extra {
		w.Header().Set(k, val)
	}
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func computeCriteriaHash(tenantID string, req AuditZipRequest) string {
	payload := struct {
		Tenant    string   `json:"tenant"`
		From      string   `json:"from"`
		To        string   `json:"to"`
		Partner   *string  `json:"partner"`
		MinAmount *float64 `json:"minAmount"`
		MaxAmount *float64 `json:"maxAmount"`
		Format    string   `json:"format"`
	}{
		Tenant:    tenantID,
		From:      req.From.Time.Format("2006-01-02"),
		To:        req.To.Time.Format("2006-01-02"),
		Partner:   req.Partner,
		MinAmount: req.MinAmount,
		MaxAmount: req.MaxAmount,
		Format:    string(req.Format),
	}
	b, _ := json.Marshal(payload)
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}

func conflictMessage(e ConflictErr) string {
	switch e.Reason {
	case IdempotencyBodyMismatch:
		return "idempotency key already used with different payload"
	case DuplicateJob:
		return "duplicate request exists for the same criteria"
	case NotCancelable:
		return "job is not cancelable in current state"
	default:
		return "duplicate request"
	}
}

func deref(p *string) string {
	if p == nil {
		return ""
	}
	return *p
}

func (s Service) decorateJob(job AuditZipJob, corrID string) AuditZipJob {
	if job.Error != nil {
		job.Error.CorrId = corrID
	}
	return job
}

func formatRetryAfter(d time.Duration) string {
	seconds := toRetrySeconds(d)
	if seconds < 1 {
		seconds = 1
	}
	return fmt.Sprintf("%d", seconds)
}

func toRetrySeconds(d time.Duration) int {
	if d <= 0 {
		return 1
	}
	return int(d.Seconds())
}

func (s Service) appendAudit(ctx context.Context, tenantID, corrID, action, criteriaHash string) error {
	if s.audit == nil {
		return nil
	}
	entry := AuditLog{
		AuditID:      newID(),
		CorrID:       corrID,
		TenantID:     tenantID,
		Actor:        "system",
		Action:       action,
		CriteriaHash: criteriaHash,
		Ts:           time.Now().UTC(),
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
