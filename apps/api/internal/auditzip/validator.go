package auditzip

import (
	"fmt"
	"time"
)

func ValidateRequest(req AuditZipRequest) []ValidationErrorItem {
	errs := make([]ValidationErrorItem, 0)
	if req.From == "" || req.To == "" {
		errs = append(errs, ValidationErrorItem{Code: "AUDIT-REQ-001", Path: "from/to", Message: "from and to dates are required"})
		return errs
	}
	from, err := parseDate(req.From)
	if err != nil {
		errs = append(errs, ValidationErrorItem{Code: "AUDIT-REQ-002", Path: "from", Message: err.Error()})
	}
	to, err2 := parseDate(req.To)
	if err2 != nil {
		errs = append(errs, ValidationErrorItem{Code: "AUDIT-REQ-003", Path: "to", Message: err2.Error()})
	}
	if err == nil && err2 == nil && to.Before(from) {
		errs = append(errs, ValidationErrorItem{Code: "AUDIT-REQ-004", Path: "to", Message: "to must be on or after from"})
	}
	if req.Format != "zip" {
		errs = append(errs, ValidationErrorItem{Code: "AUDIT-REQ-005", Path: "format", Message: "format must be zip"})
	}
	if req.Partner != nil && len(*req.Partner) > 140 {
		errs = append(errs, ValidationErrorItem{Code: "AUDIT-REQ-006", Path: "partner", Message: "partner too long"})
	}
	return errs
}

func parseDate(s string) (time.Time, error) {
	t, err := time.Parse("2006-01-02", s)
	if err != nil {
		return time.Time{}, fmt.Errorf("invalid date: %w", err)
	}
	return t, nil
}
