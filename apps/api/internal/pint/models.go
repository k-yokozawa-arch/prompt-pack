package pint

import "time"

// ValidationResult extends the generated ValidationResponse with computed totals.
// This is used internally for validation processing.
type ValidationResult struct {
Valid  bool                  `json:"valid"`
Errors []ValidationErrorItem `json:"errors"`
Totals Totals                `json:"totals,omitempty"`
}

// Totals holds computed invoice totals.
type Totals struct {
Subtotal   float64 `json:"subtotal"`
Tax        float64 `json:"tax"`
GrandTotal float64 `json:"grandTotal"`
}

// AuditLog represents an audit trail entry for invoice operations.
// This extends the generated AuditEntry with additional hash chain fields.
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
