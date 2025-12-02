package auditzip

import (
	"math"
	"time"
)

func ValidateRequest(req AuditZipRequest, cfg Config) ([]ValidationErrorItem, *SplitHint) {
	errs := make([]ValidationErrorItem, 0)
	if req.From.Time.IsZero() || req.To.Time.IsZero() {
		errs = append(errs, ValidationErrorItem{Code: "AUDIT-REQ-001", Path: "from/to", Message: "from and to are required"})
		return errs, nil
	}

	from := req.From.Time
	to := req.To.Time
	if to.Before(from) {
		errs = append(errs, ValidationErrorItem{Code: "AUDIT-REQ-004", Path: "to", Message: "to must be on or after from"})
	}
	if req.Format != Zip {
		errs = append(errs, ValidationErrorItem{Code: "AUDIT-REQ-005", Path: "format", Message: "format must be zip"})
	}
	if req.Partner != nil && len(*req.Partner) > 140 {
		errs = append(errs, ValidationErrorItem{Code: "AUDIT-REQ-006", Path: "partner", Message: "partner too long"})
	}
	if req.MinAmount != nil && *req.MinAmount < 0 {
		errs = append(errs, ValidationErrorItem{Code: "AUDIT-REQ-007", Path: "minAmount", Message: "minAmount must be >= 0"})
	}
	if req.MaxAmount != nil && *req.MaxAmount < 0 {
		errs = append(errs, ValidationErrorItem{Code: "AUDIT-REQ-008", Path: "maxAmount", Message: "maxAmount must be >= 0"})
	}
	if req.MinAmount != nil && req.MaxAmount != nil && *req.MinAmount > *req.MaxAmount {
		errs = append(errs, ValidationErrorItem{Code: "AUDIT-REQ-009", Path: "minAmount/maxAmount", Message: "minAmount must be <= maxAmount"})
	}
	if len(errs) > 0 {
		return errs, nil
	}

	if hint := splitHintIfNeeded(from, to, cfg); hint != nil {
		return nil, hint
	}
	return errs, nil
}

func splitHintIfNeeded(from, to time.Time, cfg Config) *SplitHint {
	if cfg.MaxRangeDays == 0 {
		return nil
	}
	rangeDays := int(to.Sub(from).Hours()/24) + 1
	if rangeDays <= cfg.MaxRangeDays {
		return nil
	}
	chunks := int(math.Ceil(float64(rangeDays) / float64(cfg.MaxRangeDays)))
	approx := math.Ceil(cfg.EstimatedMBPerDay * float64(rangeDays) / float64(chunks))
	return &SplitHint{
		Chunks:       chunks,
		ApproxSizeMB: approx,
	}
}
