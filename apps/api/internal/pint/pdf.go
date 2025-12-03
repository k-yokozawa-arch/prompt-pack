package pint

import (
"bytes"
"context"
"encoding/base64"
"fmt"
"html/template"
"net/url"
"time"

"github.com/chromedp/cdproto/page"
"github.com/chromedp/chromedp"
)

// PDFRenderer renders invoice PDFs via headless Chromium.
type PDFRenderer struct {
cfg Config
}

func NewPDFRenderer(cfg Config) PDFRenderer {
return PDFRenderer{cfg: cfg}
}

// Render builds an HTML from draft/totals and prints it to PDF. If Chromium is
// unavailable, it returns an error so the caller can decide to retry or skip.
func (r PDFRenderer) Render(ctx context.Context, draft InvoiceDraft, totals Totals) ([]byte, error) {
html, err := r.renderHTML(draft, totals)
if err != nil {
return nil, fmt.Errorf("render html: %w", err)
}

allocOpts := append(chromedp.DefaultExecAllocatorOptions[:],
chromedp.Flag("headless", true),
chromedp.Flag("disable-gpu", true),
chromedp.Flag("no-sandbox", true),
)
if r.cfg.PDFChromiumPath != "" {
allocOpts = append(allocOpts, chromedp.ExecPath(r.cfg.PDFChromiumPath))
}

allocCtx, cancelAlloc := chromedp.NewExecAllocator(ctx, allocOpts...)
defer cancelAlloc()

ctxTimeout := r.cfg.PDFTimeout
if ctxTimeout <= 0 {
ctxTimeout = 15 * time.Second
}
runCtx, cancelRun := chromedp.NewContext(allocCtx)
defer cancelRun()
runCtx, cancelTimeout := context.WithTimeout(runCtx, ctxTimeout)
defer cancelTimeout()

var pdfBuf []byte
dataURL := "data:text/html," + url.PathEscape(html)
err = chromedp.Run(runCtx,
chromedp.Navigate(dataURL),
chromedp.ActionFunc(func(ctx context.Context) error {
buf, _, perr := page.PrintToPDF().WithPrintBackground(true).Do(ctx)
if perr == nil {
pdfBuf = buf
}
return perr
}),
)
if err != nil {
return nil, fmt.Errorf("chromedp run failed: %w", err)
}
return pdfBuf, nil
}

// pdfDraftData is a struct for template rendering with string types
type pdfDraftData struct {
Supplier      pdfPartyData
Customer      pdfPartyData
IssueDate     string
DueDate       string
Notes         string
Currency      string
InvoiceNumber string
Lines         []pdfLineData
}

type pdfPartyData struct {
Name        string
TaxId       string
Postal      string
Address     string
CountryCode string
}

type pdfLineData struct {
Description string
Quantity    float64
UnitCode    string
UnitPrice   float64
TaxCategory string
TaxRate     float64
}

func convertDraftForPDF(draft InvoiceDraft) pdfDraftData {
notes := ""
if draft.Notes != nil {
notes = *draft.Notes
}
invoiceNumber := ""
if draft.InvoiceNumber != nil {
invoiceNumber = *draft.InvoiceNumber
}

data := pdfDraftData{
Supplier: pdfPartyData{
Name:        draft.Supplier.Name,
TaxId:       draft.Supplier.TaxId,
Postal:      draft.Supplier.Postal,
Address:     draft.Supplier.Address,
CountryCode: string(draft.Supplier.CountryCode),
},
Customer: pdfPartyData{
Name:        draft.Customer.Name,
TaxId:       draft.Customer.TaxId,
Postal:      draft.Customer.Postal,
Address:     draft.Customer.Address,
CountryCode: string(draft.Customer.CountryCode),
},
IssueDate:     draft.IssueDate.String(),
DueDate:       draft.DueDate.String(),
Notes:         notes,
Currency:      string(draft.Currency),
InvoiceNumber: invoiceNumber,
}

for _, line := range draft.Lines {
data.Lines = append(data.Lines, pdfLineData{
Description: line.Description,
Quantity:    line.Quantity,
UnitCode:    string(line.UnitCode),
UnitPrice:   line.UnitPrice,
TaxCategory: string(line.TaxCategory),
TaxRate:     line.TaxRate,
})
}
return data
}

func (r PDFRenderer) renderHTML(draft InvoiceDraft, totals Totals) (string, error) {
tz, _ := time.LoadLocation(defaultString(r.cfg.PDFTimeZone, "Asia/Tokyo"))
tmpl := template.Must(template.New("invoice").Funcs(template.FuncMap{
"money": func(v float64) string {
return fmt.Sprintf("¥%s", formatNumber(v))
},
"date": func(v string) string {
t, err := time.Parse("2006-01-02", v)
if err != nil {
return v
}
return t.In(tz).Format("2006/01/02")
},
"escape": htmlEscape,
"mul":    mul,
"mul100": mul100,
}).Parse(htmlTemplate))

pdfData := convertDraftForPDF(draft)

var buf bytes.Buffer
if err := tmpl.Execute(&buf, struct {
Draft  pdfDraftData
Totals Totals
Now    string
}{
Draft:  pdfData,
Totals: totals,
Now:    time.Now().In(tz).Format("2006/01/02 15:04"),
}); err != nil {
return "", err
}
return buf.String(), nil
}

func formatNumber(v float64) string {
return template.HTMLEscapeString(fmt.Sprintf("%0.0f", v))
}

func htmlEscape(s string) string {
return template.HTMLEscapeString(s)
}

var htmlTemplate = `
<!doctype html>
<html lang="ja">
<head>
  <meta charset="utf-8" />
  <style>
    body { font-family: 'Noto Sans JP', 'Helvetica Neue', Arial, sans-serif; margin: 24px; color: #0f172a; }
    h1 { margin: 0 0 8px; }
    .meta { display: flex; justify-content: space-between; margin-bottom: 16px; }
    .card { border: 1px solid #e2e8f0; border-radius: 8px; padding: 12px; margin-bottom: 12px; }
    .row { display: flex; gap: 12px; }
    .col { flex: 1; }
    .label { font-size: 12px; color: #475569; }
    .value { font-size: 14px; margin-bottom: 4px; }
    table { width: 100%; border-collapse: collapse; margin-top: 8px; }
    th, td { padding: 8px; border-bottom: 1px solid #e2e8f0; text-align: left; }
    th { background: #f8fafc; }
    .total { text-align: right; }
  </style>
</head>
<body>
  <div class="meta">
    <h1>請求書</h1>
    <div style="text-align:right">
      <div class="label">発行日</div>
      <div class="value">{{date .Draft.IssueDate}}</div>
      <div class="label">支払期日</div>
      <div class="value">{{date .Draft.DueDate}}</div>
      <div class="label">作成日時</div>
      <div class="value">{{.Now}}</div>
    </div>
  </div>

  <div class="card">
    <div class="row">
      <div class="col">
        <div class="label">サプライヤー</div>
        <div class="value">{{.Draft.Supplier.Name}}</div>
        <div class="value">{{.Draft.Supplier.Address}}</div>
        <div class="value">{{.Draft.Supplier.Postal}}</div>
        <div class="value">{{.Draft.Supplier.CountryCode}}</div>
        <div class="value">登録番号: {{.Draft.Supplier.TaxId}}</div>
      </div>
      <div class="col">
        <div class="label">カスタマー</div>
        <div class="value">{{.Draft.Customer.Name}}</div>
        <div class="value">{{.Draft.Customer.Address}}</div>
        <div class="value">{{.Draft.Customer.Postal}}</div>
        <div class="value">{{.Draft.Customer.CountryCode}}</div>
        <div class="value">登録番号: {{.Draft.Customer.TaxId}}</div>
      </div>
    </div>
  </div>

  <table>
    <thead>
      <tr>
        <th>内容</th>
        <th>数量</th>
        <th>単価</th>
        <th>税率</th>
        <th class="total">小計</th>
      </tr>
    </thead>
    <tbody>
    {{range .Draft.Lines}}
      <tr>
        <td>{{.Description}}</td>
        <td>{{printf "%.2f" .Quantity}} {{.UnitCode}}</td>
        <td>{{money .UnitPrice}}</td>
        <td>{{printf "%.0f%%" (mul100 .TaxRate)}}</td>
        <td class="total">{{money (mul .Quantity .UnitPrice)}}</td>
      </tr>
    {{end}}
    </tbody>
  </table>

  <div style="display:flex; justify-content:flex-end; margin-top:12px;">
    <div style="min-width:200px;">
      <div class="row" style="justify-content:space-between;"><div>小計</div><div>{{money .Totals.Subtotal}}</div></div>
      <div class="row" style="justify-content:space-between;"><div>税額</div><div>{{money .Totals.Tax}}</div></div>
      <div class="row" style="justify-content:space-between; font-weight:700;"><div>合計</div><div>{{money .Totals.GrandTotal}}</div></div>
    </div>
  </div>

  {{if .Draft.Notes}}
  <div class="card">
    <div class="label">備考</div>
    <div class="value">{{.Draft.Notes}}</div>
  </div>
  {{end}}
</body>
</html>
`

// template helper functions
func mul(a, b float64) float64 { return a * b }
func mul100(v float64) float64 { return v * 100 }

// embed font as data URI if needed
func embedFont(base64Data string) string {
if base64Data == "" {
return ""
}
return fmt.Sprintf("@font-face{font-family:'Noto Sans JP';src:url('data:font/woff2;base64,%s') format('woff2');}", base64.StdEncoding.EncodeToString([]byte(base64Data)))
}

func defaultString(s, def string) string {
if s == "" {
return def
}
return s
}
