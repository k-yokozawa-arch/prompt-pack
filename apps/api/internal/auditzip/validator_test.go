package auditzip

import "testing"

func TestValidateRequestSuccess(t *testing.T) {
	req := AuditZipRequest{From: "2025-01-01", To: "2025-01-31", Format: "zip"}
	errs := ValidateRequest(req)
	if len(errs) > 0 {
		t.Fatalf("expected no errors, got %+v", errs)
	}
}

func TestValidateRequestInvalidDate(t *testing.T) {
	req := AuditZipRequest{From: "bad", To: "2025-01-01", Format: "zip"}
	errs := ValidateRequest(req)
	if len(errs) == 0 {
		t.Fatalf("expected errors")
	}
}

func TestValidateRequestOrder(t *testing.T) {
	req := AuditZipRequest{From: "2025-02-01", To: "2025-01-01", Format: "zip"}
	errs := ValidateRequest(req)
	if len(errs) == 0 {
		t.Fatalf("expected date order error")
	}
}
