package pint

import "testing"

func TestValidate_Success(t *testing.T) {
	v := Validator{Config: LoadConfig()}
	result := v.Validate(sampleDraft())
	if !result.Valid {
		t.Fatalf("expected valid, got errors %+v", result.Errors)
	}
	if result.Totals.GrandTotal <= 0 {
		t.Fatalf("expected totals, got %+v", result.Totals)
	}
}

func TestValidate_DueDateBeforeIssue(t *testing.T) {
	v := Validator{Config: LoadConfig()}
	d := sampleDraft()
	d.DueDate = "2020-01-01"
	d.IssueDate = "2020-02-01"
	result := v.Validate(d)
	if result.Valid {
		t.Fatalf("expected invalid due date")
	}
}

func TestValidate_InvalidCodes(t *testing.T) {
	v := Validator{Config: LoadConfig()}
	d := sampleDraft()
	d.Lines[0].UnitCode = "ZZZ"
	result := v.Validate(d)
	if result.Valid {
		t.Fatalf("expected invalid unit code")
	}
}

func sampleDraft() InvoiceDraft {
	return InvoiceDraft{
		IssueDate: "2024-04-01",
		DueDate:   "2024-04-30",
		Currency:  "JPY",
		Supplier: Party{
			Name:        "Alpha",
			TaxID:       "T1234567890123",
			Postal:      "1000001",
			Address:     "Tokyo",
			CountryCode: "JP",
		},
		Customer: Party{
			Name:        "Bravo",
			TaxID:       "T9876543210000",
			Postal:      "1500001",
			Address:     "Tokyo",
			CountryCode: "JP",
		},
		Lines: []LineItem{{
			Description: "Dev",
			Quantity:    10,
			UnitCode:    "EA",
			UnitPrice:   1200,
			TaxCategory: "S",
			TaxRate:     0.1,
		}},
	}
}
