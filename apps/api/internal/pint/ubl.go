package pint

import (
	"encoding/xml"
	"fmt"
)

type UBLInvoice struct {
	XMLName         xml.Name         `xml:"Invoice"`
	Xmlns           string           `xml:"xmlns,attr"`
	Cbc             string           `xml:"xmlns:cbc,attr"`
	Cac             string           `xml:"xmlns:cac,attr"`
	CustomizationID string           `xml:"cbc:CustomizationID"`
	ProfileID       string           `xml:"cbc:ProfileID"`
	ID              string           `xml:"cbc:ID"`
	IssueDate       string           `xml:"cbc:IssueDate"`
	DueDate         string           `xml:"cbc:DueDate"`
	InvoiceTypeCode string           `xml:"cbc:InvoiceTypeCode"`
	Note            string           `xml:"cbc:Note,omitempty"`
	DocumentCurrencyCode string     `xml:"cbc:DocumentCurrencyCode"`
	AccountingSupplierParty PartyWrapper `xml:"cac:AccountingSupplierParty"`
	AccountingCustomerParty PartyWrapper `xml:"cac:AccountingCustomerParty"`
	TaxTotal             TaxTotal         `xml:"cac:TaxTotal"`
	LegalMonetaryTotal   MonetaryTotal    `xml:"cac:LegalMonetaryTotal"`
	InvoiceLine          []InvoiceLine    `xml:"cac:InvoiceLine"`
}

type PartyWrapper struct {
	Party PartyType `xml:"cac:Party"`
}

type PartyType struct {
	PartyName   NameWrapper    `xml:"cac:PartyName"`
	PostalAddress Address      `xml:"cac:PostalAddress"`
	PartyTaxScheme TaxScheme   `xml:"cac:PartyTaxScheme"`
}

type NameWrapper struct {
	Name string `xml:"cbc:Name"`
}

type Address struct {
	StreetName string `xml:"cbc:StreetName"`
	PostalZone string `xml:"cbc:PostalZone"`
	Country    Country `xml:"cac:Country"`
}

type Country struct {
	IdentificationCode string `xml:"cbc:IdentificationCode"`
}

type TaxScheme struct {
	CompanyID string  `xml:"cbc:CompanyID"`
	TaxScheme TaxInfo `xml:"cac:TaxScheme"`
}

type TaxInfo struct {
	ID string `xml:"cbc:ID"`
}

type TaxTotal struct {
	TaxAmount Amount `xml:"cbc:TaxAmount"`
}

type MonetaryTotal struct {
	LineExtensionAmount Amount `xml:"cbc:LineExtensionAmount"`
	TaxExclusiveAmount  Amount `xml:"cbc:TaxExclusiveAmount"`
	TaxInclusiveAmount  Amount `xml:"cbc:TaxInclusiveAmount"`
	PayableAmount       Amount `xml:"cbc:PayableAmount"`
}

type InvoiceLine struct {
	ID                  string            `xml:"cbc:ID"`
	InvoicedQuantity    Quantity          `xml:"cbc:InvoicedQuantity"`
	LineExtensionAmount Amount            `xml:"cbc:LineExtensionAmount"`
	Item                Item              `xml:"cac:Item"`
	Price               Price             `xml:"cac:Price"`
	TaxTotal            LineTaxTotal      `xml:"cac:TaxTotal"`
}

type Quantity struct {
	UnitCode string  `xml:"unitCode,attr"`
	Value    float64 `xml:",chardata"`
}

type Amount struct {
	Currency string  `xml:"currencyID,attr"`
	Value    float64 `xml:",chardata"`
}

type Item struct {
	Description string      `xml:"cbc:Description"`
	TaxCategory TaxCategory `xml:"cac:ClassifiedTaxCategory"`
}

type TaxCategory struct {
	ID      string   `xml:"cbc:ID"`
	Percent float64  `xml:"cbc:Percent"`
	TaxScheme TaxInfo `xml:"cac:TaxScheme"`
}

type Price struct {
	PriceAmount Amount `xml:"cbc:PriceAmount"`
}

type LineTaxTotal struct {
	TaxAmount Amount `xml:"cbc:TaxAmount"`
}

// BuildUBL marshals the draft into a minimal JP PINT aligned UBL XML.
func BuildUBL(invoiceID string, draft InvoiceDraft, totals Totals) (string, error) {
	ubl := UBLInvoice{
		Xmlns:               "urn:oasis:names:specification:ubl:schema:xsd:Invoice-2",
		Cbc:                 "urn:oasis:names:specification:ubl:schema:xsd:CommonBasicComponents-2",
		Cac:                 "urn:oasis:names:specification:ubl:schema:xsd:CommonAggregateComponents-2",
		CustomizationID:     "urn:jp:pint:invoice:1.0",
		ProfileID:           "urn:peppol:bis:billing:3",
		ID:                  invoiceID,
		IssueDate:           draft.IssueDate,
		DueDate:             draft.DueDate,
		InvoiceTypeCode:     "380",
		Note:                draft.Notes,
		DocumentCurrencyCode: draft.Currency,
		AccountingSupplierParty: PartyWrapper{
			Party: PartyType{
				PartyName: NameWrapper{Name: draft.Supplier.Name},
				PostalAddress: Address{
					StreetName: draft.Supplier.Address,
					PostalZone: draft.Supplier.Postal,
					Country: Country{IdentificationCode: draft.Supplier.CountryCode},
				},
				PartyTaxScheme: TaxScheme{
					CompanyID: draft.Supplier.TaxID,
					TaxScheme: TaxInfo{ID: "VAT"},
				},
			},
		},
		AccountingCustomerParty: PartyWrapper{
			Party: PartyType{
				PartyName: NameWrapper{Name: draft.Customer.Name},
				PostalAddress: Address{
					StreetName: draft.Customer.Address,
					PostalZone: draft.Customer.Postal,
					Country: Country{IdentificationCode: draft.Customer.CountryCode},
				},
				PartyTaxScheme: TaxScheme{
					CompanyID: draft.Customer.TaxID,
					TaxScheme: TaxInfo{ID: "VAT"},
				},
			},
		},
		TaxTotal: TaxTotal{
			TaxAmount: Amount{Currency: draft.Currency, Value: totals.Tax},
		},
		LegalMonetaryTotal: MonetaryTotal{
			LineExtensionAmount: Amount{Currency: draft.Currency, Value: totals.Subtotal},
			TaxExclusiveAmount:  Amount{Currency: draft.Currency, Value: totals.Subtotal},
			TaxInclusiveAmount:  Amount{Currency: draft.Currency, Value: totals.GrandTotal},
			PayableAmount:       Amount{Currency: draft.Currency, Value: totals.GrandTotal},
		},
	}

	for i, line := range draft.Lines {
		lineSubtotal := line.Quantity * line.UnitPrice
		lineTax := lineSubtotal * line.TaxRate
		ubl.InvoiceLine = append(ubl.InvoiceLine, InvoiceLine{
			ID: fmt.Sprintf("%d", i+1),
			InvoicedQuantity: Quantity{
				UnitCode: line.UnitCode,
				Value:    line.Quantity,
			},
			LineExtensionAmount: Amount{
				Currency: draft.Currency,
				Value:    lineSubtotal,
			},
			Item: Item{
				Description: line.Description,
				TaxCategory: TaxCategory{
					ID:       line.TaxCategory,
					Percent:  line.TaxRate * 100,
					TaxScheme: TaxInfo{ID: "VAT"},
				},
			},
			Price: Price{
				PriceAmount: Amount{Currency: draft.Currency, Value: line.UnitPrice},
			},
			TaxTotal: LineTaxTotal{
				TaxAmount: Amount{Currency: draft.Currency, Value: lineTax},
			},
		})
	}

	output, err := xml.MarshalIndent(ubl, "", "  ")
	if err != nil {
		return "", fmt.Errorf("marshal UBL: %w", err)
	}
	return xml.Header + string(output), nil
}
