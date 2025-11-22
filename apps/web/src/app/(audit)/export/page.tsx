'use client'

import { useCallback, useEffect, useMemo, useRef, useState } from 'react'
import type { AuditZipJob, AuditZipRequest, AuditZipStatus } from '../../../lib/api/audit-zip'

type QueueItem = { id: string; payload: AuditZipRequest; corrId: string; tenantId: string }

const queueStorageKey = 'audit-zip-resend-queue'

const fontStyle = { fontFamily: '"IBM Plex Sans JP", "Inter", "Noto Sans JP", "Hiragino Sans", system-ui, sans-serif' }

const defaultRequest: AuditZipRequest = {
  from: new Date(Date.now() - 7 * 86400000).toISOString().slice(0, 10),
  to: new Date().toISOString().slice(0, 10),
  partner: null,
  format: 'zip',
}

export default function AuditExportPage() {
  const [form, setForm] = useState<AuditZipRequest>(defaultRequest)
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const [message, setMessage] = useState<string | null>(null)
  const [job, setJob] = useState<AuditZipJob | null>(null)
  const [queued, setQueued] = useState<QueueItem[]>([])
  const pollTimer = useRef<NodeJS.Timeout | null>(null)

  useEffect(() => {
    const saved = localStorage.getItem(queueStorageKey)
    if (saved) setQueued(JSON.parse(saved))
  }, [])

  const saveQueue = useCallback((items: QueueItem[]) => {
    setQueued(items)
    localStorage.setItem(queueStorageKey, JSON.stringify(items))
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
        const res = await postAuditZip(item.payload, item.corrId, item.tenantId)
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

  const statusBadge = useMemo(() => {
    if (!job) return null
    const color: Record<AuditZipStatus, string> = {
      queued: '#a3e635',
      running: '#22d3ee',
      succeeded: '#22c55e',
      failed: '#f97316',
      canceled: '#94a3b8',
    }
    return (
      <span style={{ display: 'inline-flex', alignItems: 'center', gap: 8, color: color[job.status] }} aria-live="polite">
        <span aria-hidden style={{ width: 10, height: 10, background: color[job.status], borderRadius: '50%' }} />
        {job.status}
      </span>
    )
  }, [job])

  const clearPoll = () => {
    if (pollTimer.current) {
      clearTimeout(pollTimer.current)
      pollTimer.current = null
    }
  }

  const pollJob = useCallback(
    async (jobId: string) => {
      try {
        const res = await fetch(`/audit/jobs/${jobId}`, {
          headers: { 'X-Correlation-Id': 'poll-' + jobId, 'X-Tenant-Id': 'demo-tenant' },
        })
        if (!res.ok) throw new Error('進捗取得に失敗しました')
        const data = (await res.json()) as AuditZipJob
        setJob(data)
        if (data.status === 'queued' || data.status === 'running') {
          pollTimer.current = setTimeout(() => pollJob(jobId), 2000)
        }
      } catch (err: any) {
        setError(err.message)
      }
    },
    []
  )

  useEffect(() => () => clearPoll(), [])

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault()
    setLoading(true)
    setError(null)
    setMessage(null)
    setJob(null)
    clearPoll()
    const corrId = crypto.randomUUID ? crypto.randomUUID() : String(Date.now())
    const tenantId = 'demo-tenant'
    try {
      const res = await postAuditZip(form, corrId, tenantId)
      if (!res.ok) {
        const text = await res.text()
        throw new Error(text || 'ジョブ投入に失敗しました')
      }
      const data = (await res.json()) as AuditZipJob
      setJob(data)
      setMessage('ジョブを受け付けました')
      pollJob(data.jobId)
    } catch (err: any) {
      if (!navigator.onLine) {
        enqueue({ id: corrId, payload: form, corrId, tenantId })
        setMessage('オフラインのため再送キューに追加しました')
      } else {
        setError(err.message)
      }
    } finally {
      setLoading(false)
    }
  }

  return (
    <div style={{ ...fontStyle, background: 'radial-gradient(circle at 20% 20%, #0ea5e9 0, transparent 20%), #020617', minHeight: '100vh', padding: '2rem' }}>
      <main aria-labelledby="title" style={{ maxWidth: 960, margin: '0 auto', color: '#e5e7eb', display: 'grid', gap: '1rem' }}>
        <header>
          <p style={{ color: '#38bdf8', letterSpacing: '.08em', fontSize: '.85rem', margin: 0 }}>Audit ZIP</p>
          <h1 id="title" style={{ margin: 0, fontSize: '1.9rem' }}>電帳法監査ZIP生成</h1>
          <p style={{ color: '#94a3b8' }}>p95 10s でジョブ投入、バックグラウンド p95 60s で完了を目指します。</p>
        </header>

        <section aria-label="ステータス" style={cardStyle}>
          {message && <div role="status" style={infoStyle}>{message}</div>}
          {error && <div role="alert" style={errorStyle}>{error}</div>}
          {!!queued.length && (
            <div role="status" style={queueStyle}>
              オフライン再送待ち {queued.length} 件
              <button onClick={flushQueue} style={pillBtnStyle}>再送試行</button>
            </div>
          )}
          {job && (
            <div style={{ marginTop: '.5rem', display: 'flex', gap: '1rem', alignItems: 'center', flexWrap: 'wrap' }}>
              <div>Job ID: <code>{job.jobId}</code></div>
              {statusBadge}
              {job.result?.signedUrl && (
                <a href={job.result.signedUrl} style={linkStyle} target="_blank" rel="noreferrer">
                  署名URLを開く ({(job.result.size / 1024).toFixed(1)} KB)
                </a>
              )}
            </div>
          )}
        </section>

        <form onSubmit={handleSubmit} style={cardStyle}>
          <fieldset style={{ border: 'none', padding: 0, margin: 0 }}>
            <legend style={legendStyle}>抽出条件</legend>
            <div style={gridCols(3)}>
              <LabeledInput label="From" type="date" value={form.from} onChange={(v) => setForm({ ...form, from: v })} />
              <LabeledInput label="To" type="date" value={form.to} onChange={(v) => setForm({ ...form, to: v })} />
              <LabeledInput label="取引先コード (任意)" value={form.partner || ''} onChange={(v) => setForm({ ...form, partner: v || null })} placeholder="BRV-123" />
            </div>
            <div style={{ marginTop: '0.75rem', display: 'flex', gap: '1rem', alignItems: 'center', flexWrap: 'wrap' }}>
              <span>フォーマット</span>
              <strong>ZIP (固定)</strong>
            </div>
          </fieldset>
          <div style={{ marginTop: '1rem', display: 'flex', gap: '1rem', alignItems: 'center', flexWrap: 'wrap' }}>
            <button type="submit" disabled={loading} style={{ ...pillBtnStyle, background: loading ? '#475569' : '#22c55e' }}>
              {loading ? '送信中...' : 'ジョブを投入'}
            </button>
            <span aria-live="polite" style={{ color: '#94a3b8' }}>Tab/Shift+Tab で移動、Enter で送信。オフラインでもキューします。</span>
          </div>
        </form>
      </main>
    </div>
  )
}

function LabeledInput(props: { label: string; value: string | number; onChange?: (v: string) => void; type?: string; placeholder?: string }) {
  const { label, value, onChange, type = 'text', placeholder } = props
  const id = `${label}-${Math.random().toString(36).slice(2, 7)}`
  return (
    <label htmlFor={id} style={{ display: 'flex', flexDirection: 'column', gap: 4 }}>
      <span style={{ color: '#cbd5e1', fontSize: '.9rem' }}>{label}</span>
      <input
        id={id}
        type={type}
        value={value}
        onChange={(e) => onChange?.(e.target.value)}
        placeholder={placeholder}
        style={{
          background: '#0f172a',
          color: '#e5e7eb',
          border: '1px solid #1f2937',
          borderRadius: 10,
          padding: '0.6rem 0.75rem',
        }}
      />
    </label>
  )
}

function gridCols(count: number): React.CSSProperties {
  return { display: 'grid', gridTemplateColumns: `repeat(${count}, minmax(0, 1fr))`, gap: '0.75rem' }
}

const cardStyle: React.CSSProperties = {
  padding: '1rem 1.25rem',
  borderRadius: 16,
  border: '1px solid #1f2937',
  background: 'linear-gradient(135deg,#0b1221,#0f172a)',
}

const legendStyle: React.CSSProperties = { color: '#e5e7eb', fontWeight: 700, marginBottom: '0.5rem' }
const pillBtnStyle: React.CSSProperties = { border: 'none', borderRadius: 999, padding: '0.65rem 1.1rem', color: '#0b1221', fontWeight: 700, cursor: 'pointer', background: '#22c55e' }
const infoStyle: React.CSSProperties = { background: '#0f766e', padding: '0.75rem', borderRadius: 10, color: '#e0f2f1' }
const errorStyle: React.CSSProperties = { background: '#7f1d1d', padding: '0.75rem', borderRadius: 10, color: '#fee2e2' }
const queueStyle: React.CSSProperties = { background: '#4338ca', padding: '0.75rem', borderRadius: 10, color: '#e0e7ff', display: 'flex', gap: '0.5rem', alignItems: 'center', justifyContent: 'space-between', flexWrap: 'wrap' }
const linkStyle: React.CSSProperties = { color: '#38bdf8', textDecoration: 'underline' }

async function postAuditZip(body: AuditZipRequest, corrId: string, tenantId: string) {
  return fetch('/audit/zip', {
    method: 'POST',
    headers: {
      'Content-Type': 'application/json',
      'X-Correlation-Id': corrId,
      'X-Tenant-Id': tenantId,
    },
    body: JSON.stringify(body),
  })
}
