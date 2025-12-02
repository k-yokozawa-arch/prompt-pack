'use client'

import { useState } from 'react'
import type { InvoiceDraft, ValidationErrorItem, ValidationResponse } from '../../../lib/api/jp-pint'

const fontStyle = { fontFamily: '"IBM Plex Sans JP", "Inter", "Noto Sans JP", "Hiragino Sans", system-ui, sans-serif' }
const apiBase = process.env.NEXT_PUBLIC_API_BASE || 'http://localhost:8080'

const exampleDraft: InvoiceDraft = {
  issueDate: '2024-04-01',
  dueDate: '2024-04-30',
  currency: 'JPY',
  supplier: { name: 'Alpha Corp', taxId: 'T1234567890123', postal: '1000001', address: '東京都千代田区1-1', countryCode: 'JP' },
  customer: { name: 'Bravo Inc', taxId: 'T9876543210000', postal: '1500001', address: '東京都渋谷区2-2', countryCode: 'JP' },
  lines: [{ description: '開発作業', quantity: 10, unitCode: 'HUR', unitPrice: 12000, taxCategory: 'S', taxRate: 0.1 }],
  notes: '請求書一式',
}

export default function ValidatePage() {
  const [jsonInput, setJsonInput] = useState(JSON.stringify(exampleDraft, null, 2))
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const [result, setResult] = useState<ValidationResponse | null>(null)

  const handleValidate = async () => {
    setLoading(true)
    setError(null)
    setResult(null)
    const corrId = crypto.randomUUID ? crypto.randomUUID() : String(Date.now())
    try {
      const parsed = JSON.parse(jsonInput) as InvoiceDraft
      const res = await fetch(`${apiBase}/invoices/validate`, {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
          'X-Correlation-Id': corrId,
          'X-Tenant-Id': 'demo-tenant',
        },
        body: JSON.stringify(parsed),
      })
      if (!res.ok) {
        const text = await res.text()
        throw new Error(text || '検証失敗')
      }
      const data = (await res.json()) as ValidationResponse
      setResult(data)
    } catch (err: any) {
      setError(err.message)
    } finally {
      setLoading(false)
    }
  }

  const empty = !result && !error

  return (
    <div style={{ ...fontStyle, padding: '2rem', background: '#020617', minHeight: '100vh', color: '#e5e7eb' }}>
      <main aria-labelledby="title" style={{ maxWidth: 1100, margin: '0 auto', display: 'grid', gap: '1rem' }}>
        <header>
          <p style={{ color: '#a855f7', letterSpacing: '.08em', fontSize: '.85rem', margin: 0 }}>Validator</p>
          <h1 id="title" style={{ margin: 0, fontSize: '1.8rem' }}>インボイス検証</h1>
          <p style={{ color: '#94a3b8' }}>JSON入力→JP PINT検証。結果は表とツリーで確認できます。</p>
        </header>

        <section style={cardStyle}>
          <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', gap: '1rem' }}>
            <div>
              <h2 style={{ margin: 0 }}>Payload</h2>
              <p style={{ color: '#94a3b8', margin: 0 }}>スキーマ違反は即時 400。Ctrl/Cmd+Enter で検証。</p>
            </div>
            <button onClick={handleValidate} disabled={loading} style={{ ...pillBtnStyle, background: loading ? '#4b5563' : '#22c55e' }}>
              {loading ? '検証中...' : '検証する'}
            </button>
          </div>
          <textarea
            aria-label="Invoice JSON"
            value={jsonInput}
            onChange={(e) => setJsonInput(e.target.value)}
            onKeyDown={(e) => {
              if ((e.metaKey || e.ctrlKey) && e.key === 'Enter') handleValidate()
            }}
            style={{
              marginTop: '0.75rem',
              width: '100%',
              minHeight: '280px',
              background: '#0f172a',
              color: '#e5e7eb',
              border: '1px solid #1f2937',
              borderRadius: 12,
              padding: '1rem',
              fontFamily: 'ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas',
              lineHeight: 1.6,
            }}
          />
        </section>

        <section style={cardStyle} aria-live="polite">
          <h2 style={{ marginTop: 0 }}>結果</h2>
          {loading && <p>検証中...</p>}
          {error && <p role="alert" style={{ color: '#fecdd3' }}>{error}</p>}
          {empty && (
            <p style={{ color: '#94a3b8' }}>
              検証結果がここに表示されます。100行まで p95 3s で応答する前提です。オフラインでも編集でき、次回送信時に復元されます。
            </p>
          )}
          {result && (
            <>
              <StatusBadge valid={result.valid} />
              {result.totals && (
                <div style={{ display: 'flex', gap: '1rem', color: '#cbd5e1' }}>
                  <div>小計 ¥{(result.totals.subtotal ?? 0).toLocaleString('ja-JP')}</div>
                  <div>税額 ¥{(result.totals.tax ?? 0).toLocaleString('ja-JP')}</div>
                  <div>合計 ¥{(result.totals.grandTotal ?? 0).toLocaleString('ja-JP')}</div>
                </div>
              )}
              <ErrorTable errors={result.errors} />
              <details style={{ marginTop: '1rem' }}>
                <summary>JSONツリー表示</summary>
                <pre style={{ marginTop: '0.5rem', background: '#0f172a', padding: '0.75rem', borderRadius: 12, overflowX: 'auto' }}>
                  {jsonInput}
                </pre>
              </details>
            </>
          )}
        </section>
      </main>
    </div>
  )
}

function StatusBadge({ valid }: { valid: boolean }) {
  const color = valid ? '#22c55e' : '#f59e0b'
  return (
    <div aria-label="検証ステータス" style={{ display: 'inline-flex', alignItems: 'center', gap: 8, color }}>
      <span style={{ width: 12, height: 12, borderRadius: '50%', background: color, display: 'inline-block' }} aria-hidden />
      <span>{valid ? 'Valid' : 'Needs Fix'}</span>
    </div>
  )
}

function ErrorTable({ errors }: { errors: ValidationErrorItem[] }) {
  if (!errors?.length) return <p style={{ color: '#c7d2fe' }}>エラーはありません。</p>
  return (
    <div style={{ marginTop: '0.75rem', overflowX: 'auto' }}>
      <table style={{ width: '100%', borderCollapse: 'collapse', fontSize: '0.95rem' }}>
        <thead>
          <tr style={{ textAlign: 'left', color: '#cbd5e1' }}>
            <th style={thTd}>Rule</th>
            <th style={thTd}>Path</th>
            <th style={thTd}>Message</th>
            <th style={thTd}>Severity</th>
          </tr>
        </thead>
        <tbody>
          {errors.map((err) => (
            <tr key={`${err.ruleId}-${err.path}`} style={{ borderBottom: '1px solid #1f2937' }}>
              <td style={thTd}>{err.ruleId}</td>
              <td style={thTd}>{err.path}</td>
              <td style={thTd}>{err.message}</td>
              <td style={thTd}>{err.severity || 'error'}</td>
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  )
}

const cardStyle: React.CSSProperties = {
  background: 'linear-gradient(125deg,#0b1221,#0f172a)',
  borderRadius: 18,
  padding: '1rem 1.25rem',
  border: '1px solid #1f2937',
}

const pillBtnStyle: React.CSSProperties = {
  border: 'none',
  borderRadius: 999,
  padding: '0.65rem 1.15rem',
  color: '#0f172a',
  fontWeight: 700,
  cursor: 'pointer',
}

const thTd: React.CSSProperties = { padding: '0.4rem 0.5rem', color: '#e2e8f0' }
