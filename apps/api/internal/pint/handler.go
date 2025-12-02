package pint

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"time"
)

// Service wires config, validation, storage, and audit into HTTP handlers.
type Service struct {
	cfg       Config
	validator Validator
	storage   Storage
	audit     AuditRecorder
	logger    *slog.Logger
	pdf       PDFRenderer
}

func NewService(cfg Config, storage Storage, audit AuditRecorder, logger *slog.Logger) Service {
	return Service{
		cfg:       cfg,
		validator: Validator{Config: cfg},
		storage:   storage,
		audit:     audit,
		logger:    logger,
		pdf:       NewPDFRenderer(cfg),
	}
}

// ValidateInvoice matches POST /invoices/validate
func (s Service) ValidateInvoice(w http.ResponseWriter, r *http.Request) {
	ctx, corrID, tenantID, err := withRequestContext(r)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"code": "BAD_REQUEST", "message": err.Error()})
		return
	}
	logger := CorrelationLogger(s.logger, corrID, tenantID)

	draft, err := decodeDraft(r.Body)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"code": "BAD_REQUEST", "message": err.Error()})
		return
	}
	result := s.validator.Validate(draft)
	if err := s.appendAudit(ctx, tenantID, corrID, "invoice.validate"); err != nil {
		logger.Warn("audit append failed", "error", err)
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"valid":  result.Valid,
		"errors": result.Errors,
		"totals": result.Totals,
	})
}

// IssueInvoice matches POST /invoices
func (s Service) IssueInvoice(w http.ResponseWriter, r *http.Request) {
	ctx, corrID, tenantID, err := withRequestContext(r)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"code": "BAD_REQUEST", "message": err.Error()})
		return
	}
	logger := CorrelationLogger(s.logger, corrID, tenantID)

	draft, err := decodeDraft(r.Body)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"code": "BAD_REQUEST", "message": err.Error()})
		return
	}
	validation := s.validator.Validate(draft)
	if !validation.Valid {
		writeJSON(w, http.StatusBadRequest, map[string]any{
			"errors": validation.Errors,
		})
		return
	}

	invoiceID := newID()
	xmlBody, err := BuildUBL(invoiceID, draft, validation.Totals)
	if err != nil {
		logger.Error("ubl build failed", "error", err)
		writeJSON(w, http.StatusInternalServerError, map[string]any{
			"code":      "INTERNAL_ERROR",
			"message":   "failed to generate UBL XML",
			"retryable": true,
		})
		return
	}

	xmlKey := fmt.Sprintf("%s/invoices/%s/invoice.xml", tenantID, invoiceID)
	if err := s.storage.PutObject(ctx, xmlKey, []byte(xmlBody), "application/xml"); err != nil {
		logger.Error("store xml failed", "error", err)
		writeJSON(w, http.StatusInternalServerError, map[string]any{
			"code":      "INTERNAL_ERROR",
			"message":   "storage error",
			"retryable": true,
		})
		return
	}
	xmlURL, _ := s.storage.GetSignedURL(ctx, xmlKey, s.cfg.SignURLTTL)

	var pdfURL string
	if s.cfg.PDFEnabled {
		pdfKey := fmt.Sprintf("%s/invoices/%s/invoice.pdf", tenantID, invoiceID)
		if pdfBytes, pdfErr := s.pdf.Render(ctx, draft, validation.Totals); pdfErr == nil {
			if err := s.storage.PutObject(ctx, pdfKey, pdfBytes, "application/pdf"); err != nil {
				logger.Warn("store pdf failed", "error", err)
			} else {
				pdfURL, _ = s.storage.GetSignedURL(ctx, pdfKey, s.cfg.SignURLTTL)
			}
		} else {
			logger.Warn("pdf render failed", "error", pdfErr)
		}
	}

	if err := s.appendAudit(ctx, tenantID, corrID, "invoice.issue"); err != nil {
		logger.Warn("audit append failed", "error", err)
	}

	writeJSONStatus(w, http.StatusCreated, map[string]any{
		"invoiceId": invoiceID,
		"status":    "issued",
		"xmlUrl":    xmlURL,
		"pdfUrl":    pdfURL,
		"expiresAt": time.Now().Add(s.cfg.SignURLTTL).UTC().Format(time.RFC3339),
	})
}

// GetInvoice matches GET /invoices/{id}
func (s Service) GetInvoice(w http.ResponseWriter, r *http.Request, id string) {
	ctx, corrID, tenantID, err := withRequestContext(r)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"code": "BAD_REQUEST", "message": err.Error()})
		return
	}
	logger := CorrelationLogger(s.logger, corrID, tenantID)

	xmlKey := fmt.Sprintf("%s/invoices/%s/invoice.xml", tenantID, id)
	meta, err := s.storage.Head(ctx, xmlKey)
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"code": "NOT_FOUND", "message": "invoice not found"})
		return
	}

	xmlURL, _ := s.storage.GetSignedURL(ctx, xmlKey, s.cfg.SignURLTTL)
	pdfKey := fmt.Sprintf("%s/invoices/%s/invoice.pdf", tenantID, id)
	pdfURL, _ := s.storage.GetSignedURL(ctx, pdfKey, s.cfg.SignURLTTL)

	record := InvoiceRecord{
		InvoiceIssued: InvoiceIssued{
			InvoiceID: id,
			Status:    "issued",
			XMLURL:    xmlURL,
			PDFURL:    pdfURL,
			ExpiresAt: time.Now().Add(s.cfg.SignURLTTL),
		},
		CreatedAt: meta.UpdatedAt,
		UpdatedAt: meta.UpdatedAt,
		Audit: AuditLog{
			CorrID:   corrID,
			TenantID: tenantID,
		},
	}
	if err := s.appendAudit(ctx, tenantID, corrID, "invoice.get"); err != nil {
		logger.Warn("audit append failed", "error", err)
	}
	writeJSON(w, http.StatusOK, record)
}

func decodeDraft(body io.ReadCloser) (InvoiceDraft, error) {
	defer body.Close()
	var draft InvoiceDraft
	dec := json.NewDecoder(body)
	if err := dec.Decode(&draft); err != nil {
		return draft, fmt.Errorf("invalid JSON: %w", err)
	}
	return draft, nil
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
