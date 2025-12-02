package auditzip

import "time"

// AuditLog represents append-only audit entries with hash chaining.
type AuditLog struct {
	AuditID      string    `json:"auditId"`
	CorrID       string    `json:"corrId"`
	TenantID     string    `json:"tenantId"`
	Actor        string    `json:"actor"`
	Action       string    `json:"action"`
	CriteriaHash string    `json:"criteriaHash"`
	Ts           time.Time `json:"timestamp"`
	Hash         string    `json:"hash"`
	PrevHash     string    `json:"prevHash"`
}
