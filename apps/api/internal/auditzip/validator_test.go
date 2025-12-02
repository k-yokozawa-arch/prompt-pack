package auditzip

import (
	"testing"
	"time"

	openapi_types "github.com/oapi-codegen/runtime/types"
)

func TestValidateRequestSuccess(t *testing.T) {
	req := AuditZipRequest{
		From:   openapi_types.Date{Time: time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)},
		To:     openapi_types.Date{Time: time.Date(2025, 1, 31, 0, 0, 0, 0, time.UTC)},
		Format: Zip,
	}
	errs, hint := ValidateRequest(req, LoadConfig())
	if len(errs) > 0 || hint != nil {
		t.Fatalf("expected no errors or hint, got errs=%v hint=%v", errs, hint)
	}
}

func TestValidateRequestInvalidDate(t *testing.T) {
	req := AuditZipRequest{Format: Zip}
	errs, _ := ValidateRequest(req, LoadConfig())
	if len(errs) == 0 {
		t.Fatalf("expected errors for missing dates")
	}
}

func TestValidateRequestOrder(t *testing.T) {
	req := AuditZipRequest{
		From:   openapi_types.Date{Time: time.Date(2025, 2, 1, 0, 0, 0, 0, time.UTC)},
		To:     openapi_types.Date{Time: time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)},
		Format: Zip,
	}
	errs, _ := ValidateRequest(req, LoadConfig())
	if len(errs) == 0 {
		t.Fatalf("expected date order error")
	}
}

func TestValidateRequestSplitHint(t *testing.T) {
	cfg := LoadConfig()
	cfg.MaxRangeDays = 1
	req := AuditZipRequest{
		From:   openapi_types.Date{Time: time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)},
		To:     openapi_types.Date{Time: time.Date(2025, 1, 3, 0, 0, 0, 0, time.UTC)},
		Format: Zip,
	}
	errs, hint := ValidateRequest(req, cfg)
	if len(errs) != 0 {
		t.Fatalf("unexpected validation errors: %+v", errs)
	}
	if hint == nil || hint.Chunks < 2 {
		t.Fatalf("expected split hint, got %+v", hint)
	}
}
