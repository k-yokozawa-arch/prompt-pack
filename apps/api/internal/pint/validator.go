package pint

import (
	"fmt"
	"math"
	"strings"
	"time"

	openapi_types "github.com/oapi-codegen/runtime/types"
)

type Validator struct {
	Config Config
}

func (v Validator) Validate(draft InvoiceDraft) ValidationResult {
	errors := make([]ValidationErrorItem, 0)

	if draft.Supplier.Name == "" || draft.Customer.Name == "" {
		errors = append(errors, errItem("JP-PINT-REQ-001", "supplier.name/customer.name", "Supplier and customer names are required"))
	}

	// Validate dates - IssueDate and DueDate are openapi_types.Date
	issueDateStr := draft.IssueDate.String()
	dueDateStr := draft.DueDate.String()
	if issueDateStr == "0001-01-01" || dueDateStr == "0001-01-01" {
		errors = append(errors, errItem("JP-PINT-REQ-002", "issueDate/dueDate", "Issue and due dates are required"))
	}

	issue := dateToTime(draft.IssueDate)
	due := dateToTime(draft.DueDate)
	if !issue.IsZero() && !due.IsZero() && due.Before(issue) {
		errors = append(errors, errItem("JP-PINT-MATH-002", "dueDate", "Due date must be on or after issue date"))
	}

	if draft.Currency != JPY {
		errors = append(errors, errItem("JP-PINT-REQ-005", "currency", "Only JPY is supported in this version"))
	}

	if len(draft.Lines) == 0 {
		errors = append(errors, errItem("JP-PINT-REQ-006", "lines", "At least one line item is required"))
	}
	if len(draft.Lines) > v.Config.MaxLines {
		errors = append(errors, errItem("JP-PINT-LIMIT-001", "lines", fmt.Sprintf("Too many lines (max %d)", v.Config.MaxLines)))
	}

	var subtotal, taxTotal float64
	for i, line := range draft.Lines {
		path := fmt.Sprintf("lines[%d]", i)
		if strings.TrimSpace(line.Description) == "" {
			errors = append(errors, errItem("JP-PINT-REQ-007", path+".description", "Description is required"))
		}
		if len(line.Description) > v.Config.MaxDescription {
			errors = append(errors, errItem("JP-PINT-LIMIT-002", path+".description", "Description too long"))
		}
		if line.Quantity <= 0 {
			errors = append(errors, errItem("JP-PINT-MATH-003", path+".quantity", "Quantity must be positive"))
		}
		if line.UnitPrice < 0 {
			errors = append(errors, errItem("JP-PINT-MATH-004", path+".unitPrice", "Unit price must be non-negative"))
		}
		if !contains(v.Config.ValidUnitCodes, string(line.UnitCode)) {
			errors = append(errors, errItem("JP-PINT-CODE-001", path+".unitCode", "Invalid unit code"))
		}
		if !contains(v.Config.ValidTaxCategory, string(line.TaxCategory)) {
			errors = append(errors, errItem("JP-PINT-CODE-002", path+".taxCategory", "Invalid tax category"))
		}
		if line.TaxRate < 0 || line.TaxRate > 1 {
			errors = append(errors, errItem("JP-PINT-MATH-005", path+".taxRate", "Tax rate must be between 0 and 1"))
		}

		lineSubtotal := round(line.Quantity*line.UnitPrice, 2)
		lineTax := round(lineSubtotal*line.TaxRate, 2)
		subtotal += lineSubtotal
		taxTotal += lineTax
	}

	grandTotal := round(subtotal+taxTotal, 2)

	result := ValidationResult{
		Valid:  len(errors) == 0,
		Errors: errors,
		Totals: Totals{
			Subtotal:   subtotal,
			Tax:        taxTotal,
			GrandTotal: grandTotal,
		},
	}
	return result
}

func errItem(ruleID, path, message string) ValidationErrorItem {
	return ValidationErrorItem{
		Code:    ruleID,
		Path:    path,
		Message: message,
		RuleId:  ruleID,
	}
}

// dateToTime converts openapi_types.Date to time.Time
func dateToTime(d openapi_types.Date) time.Time {
	return d.Time
}

func round(val float64, places int) float64 {
	p := math.Pow(10, float64(places))
	return math.Round(val*p) / p
}

func contains(list []string, value string) bool {
	for _, item := range list {
		if item == value {
			return true
		}
	}
	return false
}
