package pint

import "time"

// InvoiceDraft mirrors the OpenAPI contract and is intended to be generated.
type InvoiceDraft struct {
	InvoiceNumber string     `json:"invoiceNumber,omitempty"`
	Supplier      Party      `json:"supplier"`
	Customer      Party      `json:"customer"`
	IssueDate     string     `json:"issueDate"`
	DueDate       string     `json:"dueDate"`
	Currency      string     `json:"currency"`
	Notes         string     `json:"notes,omitempty"`
	Lines         []LineItem `json:"lines"`
}

type Party struct {
	Name        string `json:"name"`
	TaxID       string `json:"taxId"`
	Postal      string `json:"postal"`
	Address     string `json:"address"`
	CountryCode string `json:"countryCode"`
}

type LineItem struct {
	Description string  `json:"description"`
	Quantity    float64 `json:"quantity"`
	UnitCode    string  `json:"unitCode"`
	UnitPrice   float64 `json:"unitPrice"`
	TaxCategory string  `json:"taxCategory"`
	TaxRate     float64 `json:"taxRate"`
}

type ValidationErrorItem struct {
	Code     string `json:"code"`
	Path     string `json:"path"`
	Message  string `json:"message"`
	RuleID   string `json:"ruleId"`
	Severity string `json:"severity,omitempty"`
}

type ValidationResult struct {
	Valid  bool                  `json:"valid"`
	Errors []ValidationErrorItem `json:"errors"`
	Totals Totals                `json:"totals,omitempty"`
}

type Totals struct {
	Subtotal   float64 `json:"subtotal"`
	Tax        float64 `json:"tax"`
	GrandTotal float64 `json:"grandTotal"`
}

type InvoiceIssued struct {
	InvoiceID string    `json:"invoiceId"`
	Status    string    `json:"status"`
	XMLURL    string    `json:"xmlUrl"`
	PDFURL    string    `json:"pdfUrl,omitempty"`
	ExpiresAt time.Time `json:"expiresAt"`
}

type InvoiceRecord struct {
	InvoiceIssued
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
	Audit     AuditLog  `json:"audit"`
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
