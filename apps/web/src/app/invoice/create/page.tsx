'use client'

import { useCallback, useEffect, useMemo, useState, useId } from 'react'
import type { InvoiceDraft, LineItem } from '../../../lib/api/jp-pint'

type QueueItem = { id: string; payload: InvoiceDraft; corrId: string; tenantId: string }

const queueStorageKey = 'jp-pint-resend-queue'
const apiBase = process.env.NEXT_PUBLIC_API_BASE || 'http://localhost:8080'

const emptyDraft: InvoiceDraft = {
  currency: 'JPY',
  issueDate: '',
  dueDate: '',
  supplier: { name: '', taxId: '', postal: '', address: '', countryCode: 'JP' },
  customer: { name: '', taxId: '', postal: '', address: '', countryCode: 'JP' },
  lines: [{ description: '', quantity: 1, unitCode: 'EA', unitPrice: 0, taxCategory: 'S', taxRate: 0.1 }],
}

const fontStyle = { fontFamily: '"IBM Plex Sans JP", "Inter", "Noto Sans JP", "Hiragino Sans", system-ui, sans-serif' }

export default function CreateInvoicePage() {
  const [draft, setDraft] = useState<InvoiceDraft>(() => emptyDraft)
  const [loading, setLoading] = useState(false)
  const [message, setMessage] = useState<string | null>(null)
  const [error, setError] = useState<string | null>(null)
  const [queued, setQueued] = useState<QueueItem[]>([])

  useEffect(() => {
    const today = new Date()
    const due = new Date(today.getTime() + 7 * 86400000)
    setDraft((prev) => ({ ...prev, issueDate: toISO(today), dueDate: toISO(due) }))
  }, [])

  useEffect(() => {
    const saved = typeof localStorage !== 'undefined' ? localStorage.getItem(queueStorageKey) : null
    if (saved) {
      setQueued(JSON.parse(saved))
    }
  }, [])

  const saveQueue = useCallback((items: QueueItem[]) => {
    setQueued(items)
    if (typeof localStorage !== 'undefined') {
      localStorage.setItem(queueStorageKey, JSON.stringify(items))
    }
  }, [])

  const enqueue = useCallback(
    (item: QueueItem) => {
      const next = [...queued, item]
      saveQueue(next)
    },
    [queued, saveQueue]
  )

  const flushQueue = useCallback(async () => {
    if (!queued.length || !navigator.onLine) return
    const remaining: QueueItem[] = []
    for (const item of queued) {
      try {
        const res = await postInvoice(item.payload, item.corrId, item.tenantId)
        if (!res.ok) {
          remaining.push(item)
        }
      } catch {
        remaining.push(item)
      }
    }
    saveQueue(remaining)
  }, [queued, saveQueue])

  useEffect(() => {
    window.addEventListener('online', flushQueue)
    return () => window.removeEventListener('online', flushQueue)
  }, [flushQueue])

  const totals = useMemo(() => {
    const subtotal = draft.lines.reduce((sum, line) => sum + line.quantity * line.unitPrice, 0)
    const tax = draft.lines.reduce((sum, line) => sum + line.quantity * line.unitPrice * line.taxRate, 0)
    return { subtotal, tax, grandTotal: subtotal + tax }
  }, [draft.lines])

  const handleLineChange = (index: number, key: keyof LineItem, value: string | number) => {
    const next = draft.lines.map((line, i) => (i === index ? { ...line, [key]: value } : line))
    setDraft({ ...draft, lines: next })
  }

  const addLine = () => {
    setDraft({
      ...draft,
      lines: [
        ...draft.lines,
        { description: '', quantity: 1, unitCode: 'EA', unitPrice: 0, taxCategory: 'S', taxRate: 0.1 },
      ],
    })
  }

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault()
    setLoading(true)
    setMessage(null)
    setError(null)
    const corrId = crypto.randomUUID ? crypto.randomUUID() : String(Date.now())
    const tenantId = 'demo-tenant'
    try {
      const res = await postInvoice(draft, corrId, tenantId)
      if (!res.ok) {
        const text = await res.text()
        throw new Error(text || '発行エラー')
      }
      const data = await res.json()
      setMessage(`発行成功: ${data.invoiceId}`)
    } catch (err: any) {
      if (!navigator.onLine) {
        enqueue({ id: corrId, payload: draft, corrId, tenantId })
        setMessage('オフラインのため再送キューに入れました')
      } else {
        setError(err.message)
      }
    } finally {
      setLoading(false)
    }
  }

  return (
    <div style={{ ...fontStyle, padding: '2rem', background: 'linear-gradient(135deg,#0f172a,#111827)', minHeight: '100vh' }}>
      <main aria-labelledby="title" style={{ maxWidth: 960, margin: '0 auto', background: '#0b1221', color: '#e5e7eb', padding: '1.5rem', borderRadius: '18px', boxShadow: '0 20px 80px rgba(0,0,0,0.35)' }}>
        <header style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'baseline', gap: '1rem' }}>
          <div>
            <p style={{ color: '#67e8f9', letterSpacing: '.04em', fontSize: '0.85rem' }}>JP PINT</p>
            <h1 id="title" style={{ margin: 0, fontSize: '1.8rem' }}>インボイス発行</h1>
            <p style={{ color: '#94a3b8' }}>45秒以内の発行を目指すフォーム。キーボード操作優先。</p>
          </div>
          <div aria-live="polite" style={{ textAlign: 'right' }}>
            <strong>合計</strong>
            <div>小計 ¥{totals.subtotal.toLocaleString('ja-JP')}</div>
            <div>税額 ¥{totals.tax.toLocaleString('ja-JP')}</div>
            <div>総計 ¥{totals.grandTotal.toLocaleString('ja-JP')}</div>
          </div>
        </header>

        <section aria-label="ステータス" style={{ marginTop: '1rem' }}>
          {message && <div role="status" style={{ background: '#0f766e', padding: '0.75rem', borderRadius: 8, color: '#e0f2f1' }}>{message}</div>}
          {error && <div role="alert" style={{ background: '#7f1d1d', padding: '0.75rem', borderRadius: 8, color: '#fee2e2' }}>{error}</div>}
          {!!queued.length && (
            <div role="status" style={{ background: '#4338ca', padding: '0.75rem', borderRadius: 8, marginTop: '0.5rem', color: '#e0e7ff' }}>
              オフライン再送待ち {queued.length} 件 <button onClick={flushQueue} style={pillBtnStyle}>再送試行</button>
            </div>
          )}
        </section>

        <form onSubmit={handleSubmit} style={{ marginTop: '1rem', display: 'grid', gap: '1rem' }}>
          <fieldset style={cardStyle}>
            <legend style={legendStyle}>サプライヤー</legend>
            <PartyFields prefix="supplier" party={draft.supplier} onChange={(party) => setDraft({ ...draft, supplier: party })} />
          </fieldset>
          <fieldset style={cardStyle}>
            <legend style={legendStyle}>カスタマー</legend>
            <PartyFields prefix="customer" party={draft.customer} onChange={(party) => setDraft({ ...draft, customer: party })} />
          </fieldset>
          <fieldset style={cardStyle}>
            <legend style={legendStyle}>基本情報</legend>
            <div style={gridCols(3)}>
              <LabeledInput label="発行日" type="date" value={draft.issueDate} onChange={(v) => setDraft({ ...draft, issueDate: v })} />
              <LabeledInput label="支払期日" type="date" value={draft.dueDate} onChange={(v) => setDraft({ ...draft, dueDate: v })} />
              <LabeledInput label="通貨" value={draft.currency} readOnly />
            </div>
            <LabeledInput label="備考" value={draft.notes || ''} onChange={(v) => setDraft({ ...draft, notes: v })} placeholder="振込口座や通知事項" />
          </fieldset>
          <fieldset style={cardStyle}>
            <legend style={legendStyle}>明細</legend>
            {draft.lines.map((line, i) => (
              <div key={i} style={{ marginBottom: '0.75rem', borderBottom: '1px solid #1f2937', paddingBottom: '0.75rem' }}>
                <div aria-label={`明細 ${i + 1}`} style={gridCols(4)}>
                  <LabeledInput label="内容" value={line.description} onChange={(v) => handleLineChange(i, 'description', v)} placeholder="作業内容" />
                  <LabeledInput label="数量" type="number" value={line.quantity} onChange={(v) => handleLineChange(i, 'quantity', Number(v))} min={0} />
                  <LabeledInput label="単価" type="number" value={line.unitPrice} onChange={(v) => handleLineChange(i, 'unitPrice', Number(v))} min={0} />
                  <LabeledInput label="単位コード" value={line.unitCode} onChange={(v) => handleLineChange(i, 'unitCode', v as LineItem['unitCode'])} />
                </div>
                <div style={gridCols(3)}>
                  <LabeledInput label="税率" type="number" step="0.01" value={line.taxRate} onChange={(v) => handleLineChange(i, 'taxRate', Number(v))} />
                  <LabeledInput label="税区分" value={line.taxCategory} onChange={(v) => handleLineChange(i, 'taxCategory', v as LineItem['taxCategory'])} />
                  <div aria-live="polite" style={{ paddingTop: '1.75rem', color: '#9ca3af' }}>
                    小計 ¥{(line.quantity * line.unitPrice).toLocaleString('ja-JP')}
                  </div>
                </div>
              </div>
            ))}
            <button type="button" onClick={addLine} style={pillBtnStyle}>
              明細を追加
            </button>
          </fieldset>
          <div style={{ display: 'flex', gap: '1rem', alignItems: 'center' }}>
            <button type="submit" disabled={loading} style={{ ...pillBtnStyle, background: loading ? '#475569' : '#22c55e' }}>
              {loading ? '送信中...' : '発行する'}
            </button>
            <span aria-live="polite" style={{ color: '#94a3b8' }}>
              フォームは Tab/Shift+Tab で移動、Enter で送信できます。
            </span>
          </div>
        </form>
      </main>
    </div>
  )
}

function LabeledInput(props: {
  label: string
  value: string | number
  onChange?: (v: string) => void
  type?: string
  placeholder?: string
  readOnly?: boolean
  min?: number
  step?: string
}) {
  const { label, value, onChange, type = 'text', placeholder, readOnly, min, step } = props
  const id = useId()
  return (
    <label htmlFor={id} style={{ display: 'flex', flexDirection: 'column', gap: 4 }}>
      <span style={{ color: '#cbd5e1', fontSize: '0.9rem' }}>{label}</span>
      <input
        id={id}
        type={type}
        value={value}
        min={min}
        step={step}
        readOnly={readOnly}
        placeholder={placeholder}
        onChange={(e) => onChange?.(e.target.value)}
        style={{
          background: '#111827',
          color: '#e5e7eb',
          border: '1px solid #1f2937',
          borderRadius: 10,
          padding: '0.6rem 0.75rem',
        }}
      />
    </label>
  )
}

function PartyFields({ party, onChange, prefix }: { party: InvoiceDraft['supplier']; onChange: (p: InvoiceDraft['supplier']) => void; prefix: string }) {
  return (
    <div style={gridCols(3)}>
      <LabeledInput label={`${prefix} 名称`} value={party.name} onChange={(v) => onChange({ ...party, name: v })} />
      <LabeledInput label="登録番号" value={party.taxId} onChange={(v) => onChange({ ...party, taxId: v })} />
      <LabeledInput label="郵便番号" value={party.postal} onChange={(v) => onChange({ ...party, postal: v })} />
      <LabeledInput label="住所" value={party.address} onChange={(v) => onChange({ ...party, address: v })} />
      <LabeledInput label="国コード" value={party.countryCode} readOnly />
    </div>
  )
}

function gridCols(count: number): React.CSSProperties {
  return {
    display: 'grid',
    gridTemplateColumns: `repeat(${count}, minmax(0, 1fr))`,
    gap: '0.75rem',
    alignItems: 'flex-start',
  }
}

const cardStyle: React.CSSProperties = {
  border: '1px solid #1f2937',
  borderRadius: 16,
  padding: '1rem',
  background: 'linear-gradient(145deg,#0b1221,#0f172a)',
}

const legendStyle: React.CSSProperties = {
  padding: '0 0.5rem',
  color: '#e5e7eb',
  fontWeight: 700,
}

const pillBtnStyle: React.CSSProperties = {
  background: '#14b8a6',
  border: 'none',
  color: '#0b1221',
  padding: '0.65rem 1.1rem',
  borderRadius: 999,
  fontWeight: 700,
  cursor: 'pointer',
}

async function postInvoice(body: InvoiceDraft, corrId: string, tenantId: string) {
  return fetch(`${apiBase}/invoices`, {
    method: 'POST',
    headers: {
      'Content-Type': 'application/json',
      'X-Correlation-Id': corrId,
      'X-Tenant-Id': tenantId,
    },
    body: JSON.stringify(body),
  })
}

function toISO(d: Date) {
  return d.toISOString().slice(0, 10)
}
