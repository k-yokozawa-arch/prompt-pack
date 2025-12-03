package pint

import (
"testing"
"time"

openapi_types "github.com/oapi-codegen/runtime/types"
)

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
d.DueDate = openapi_types.Date{Time: time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)}
d.IssueDate = openapi_types.Date{Time: time.Date(2020, 2, 1, 0, 0, 0, 0, time.UTC)}
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
IssueDate: openapi_types.Date{Time: time.Date(2024, 4, 1, 0, 0, 0, 0, time.UTC)},
DueDate:   openapi_types.Date{Time: time.Date(2024, 4, 30, 0, 0, 0, 0, time.UTC)},
Currency:  JPY,
Supplier: Party{
Name:        "Alpha",
TaxId:       "T1234567890123",
Postal:      "1000001",
Address:     "Tokyo",
CountryCode: JP,
},
Customer: Party{
Name:        "Bravo",
TaxId:       "T9876543210000",
Postal:      "1500001",
Address:     "Tokyo",
CountryCode: JP,
},
Lines: []LineItem{{
Description: "Dev",
Quantity:    10,
UnitCode:    EA,
UnitPrice:   1200,
TaxCategory: S,
TaxRate:     0.1,
}},
}
}
