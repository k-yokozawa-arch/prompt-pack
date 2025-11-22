package auditzip

import "time"

type AuditZipRequest struct {
	From    string  `json:"from"`
	To      string  `json:"to"`
	Partner *string `json:"partner,omitempty"`
	Format  string  `json:"format"`
}

type AuditZipResult struct {
	SignedURL string `json:"signedUrl"`
	Size      int    `json:"size"`
}

type AuditZipJob struct {
	JobID       string         `json:"jobId"`
	Status      string         `json:"status"`
	RequestedAt time.Time      `json:"requestedAt"`
	CompletedAt *time.Time     `json:"completedAt,omitempty"`
	Result      *AuditZipResult `json:"result,omitempty"`
	Error       *InternalError `json:"error,omitempty"`
}

type ValidationErrorItem struct {
	Code    string `json:"code"`
	Path    string `json:"path"`
	Message string `json:"message"`
}

type ValidationError struct {
	Errors []ValidationErrorItem `json:"errors"`
}

type InternalError struct {
	Code      string `json:"code"`
	Message   string `json:"message"`
	Retryable bool   `json:"retryable"`
}

type AuditLog struct {
	AuditID  string    `json:"auditId"`
	CorrID   string    `json:"corrId"`
	TenantID string    `json:"tenantId"`
	Actor    string    `json:"actor"`
	Action   string    `json:"action"`
	Ts       time.Time `json:"timestamp"`
	Hash     string    `json:"hash"`
	PrevHash string    `json:"prevHash"`
}
