'use client'

import { useCallback, useEffect, useId, useMemo, useRef, useState } from 'react'
import type { AuditZipJob, AuditZipRequest, AuditZipStatus, ConflictError, RequestTooLargeError, ValidationError } from '../../../lib/api/audit-zip'

type QueueItem = {
  idempotencyKey: string
  payload: AuditZipRequest
  corrId: string
  tenantId: string
  attempts: number
  nextAttemptAt: number
}

const queueStorageKey = 'audit-zip-resend-queue'
const num = (val: string | undefined, fallback: number) => {
  const parsed = Number(val)
  return Number.isFinite(parsed) ? parsed : fallback
}
const cfg = {
  apiBase: process.env.NEXT_PUBLIC_API_BASE ?? '',
  tenantId: process.env.NEXT_PUBLIC_TENANT_ID ?? 'demo-tenant',
  pollMs: num(process.env.NEXT_PUBLIC_AUDIT_POLL_MS, 1800),
  queueMax: num(process.env.NEXT_PUBLIC_AUDIT_QUEUE_MAX, 20),
  queueMaxAttempts: num(process.env.NEXT_PUBLIC_AUDIT_QUEUE_MAX_ATTEMPTS, 5),
  queueBackoffMs: num(process.env.NEXT_PUBLIC_AUDIT_QUEUE_BACKOFF_MS, 2000),
}

export default function AuditExportPage() {
  const [form, setForm] = useState<AuditZipRequest>(() => ({
    from: '',
    to: '',
    partner: null,
    minAmount: null,
    maxAmount: null,
    format: 'zip',
  }))
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const [message, setMessage] = useState<string | null>(null)
  const [job, setJob] = useState<AuditZipJob | null>(null)
  const [queued, setQueued] = useState<QueueItem[]>([])
  const pollTimer = useRef<NodeJS.Timeout | null>(null)

  useEffect(() => {
    const today = new Date()
    const from = new Date(today.getTime() - 7 * 86400000)
    setForm((prev) => ({ ...prev, from: toISO(from), to: toISO(today) }))
  }, [])

  useEffect(() => {
    const saved = typeof localStorage !== 'undefined' ? localStorage.getItem(queueStorageKey) : null
    if (saved) {
      try {
        const parsed = JSON.parse(saved) as QueueItem[]
        setQueued(parsed)
      } catch {
        setQueued([])
      }
    }
  }, [])

  const saveQueue = useCallback((items: QueueItem[]) => {
    setQueued(items)
    if (typeof localStorage !== 'undefined') {
      localStorage.setItem(queueStorageKey, JSON.stringify(items))
    }
  }, [])

  const enqueueOffline = useCallback(
    (item: QueueItem) => {
      const snapshot = [...queued]
      snapshot.push(item)
      if (snapshot.length > cfg.queueMax) {
        snapshot.shift()
      }
      saveQueue(snapshot)
    },
    [queued, saveQueue]
  )

  const clearPoll = () => {
    if (pollTimer.current) {
      clearTimeout(pollTimer.current)
      pollTimer.current = null
    }
  }

  const pollJob = useCallback(
    async (jobId: string) => {
      clearPoll()
      try {
        const corrId = newUUID()
        const res = await fetch(`${cfg.apiBase}/audit/jobs/${jobId}`, {
          headers: {
            'X-Correlation-Id': corrId,
            'X-Tenant-Id': cfg.tenantId,
          },
        })
        if (!res.ok) {
          throw new Error('進捗取得に失敗しました')
        }
        const data = (await res.json()) as AuditZipJob
        setJob(data)
        if (data.status === 'queued' || data.status === 'running') {
          pollTimer.current = setTimeout(() => pollJob(jobId), cfg.pollMs)
        }
      } catch (err: any) {
        setError(err.message ?? '進捗取得に失敗しました')
      }
    },
    [setJob]
  )

  const processQueue = useCallback(async () => {
    if (!navigator.onLine || !queued.length) return
    const now = Date.now()
    const next: QueueItem[] = []
    for (const item of queued) {
      if (item.nextAttemptAt > now) {
        next.push(item)
        continue
      }
      try {
        const res = await postAuditZip(item.payload, item.corrId, item.tenantId, item.idempotencyKey)
        if (res.ok) {
          const data = (await res.json()) as AuditZipJob
          setJob(data)
          setMessage('再送キューから送信しました')
          pollJob(data.jobId)
          continue
        }
        const payload = await safeJson(res)
        setError(handleErrorResponse(res.status, payload, res.headers.get('Retry-After') || undefined))
        const attempts = item.attempts + 1
        if (attempts >= cfg.queueMaxAttempts) {
          setError('再送キューの最大試行回数に達しました')
          continue
        }
        const backoff = cfg.queueBackoffMs * Math.pow(2, attempts - 1)
        next.push({ ...item, attempts, nextAttemptAt: Date.now() + backoff })
      } catch {
        next.push(item)
      }
    }
    saveQueue(next)
  }, [queued, saveQueue, pollJob])

  useEffect(() => {
    window.addEventListener('online', processQueue)
    return () => window.removeEventListener('online', processQueue)
  }, [processQueue])

  useEffect(() => {
    if (queued.length && navigator.onLine) {
      void processQueue()
    }
  }, [queued.length, processQueue])

  useEffect(() => () => clearPoll(), [])

  const cancelJob = async () => {
    if (!job) return
    setLoading(true)
    setError(null)
    try {
      const corrId = newUUID()
      const res = await fetch(`${cfg.apiBase}/audit/jobs/${job.jobId}?cancel=true`, {
        headers: {
          'X-Correlation-Id': corrId,
          'X-Tenant-Id': cfg.tenantId,
        },
      })
      const payload = await safeJson(res)
      if (!res.ok) {
        setError(handleErrorResponse(res.status, payload, res.headers.get('Retry-After') || undefined))
        return
      }
      const data = payload as AuditZipJob
      setJob(data)
      setMessage('キャンセルを受け付けました')
    } catch (err: any) {
      setError(err.message ?? 'キャンセルに失敗しました')
    } finally {
      setLoading(false)
    }
  }

  const handleSubmit = async (e?: React.FormEvent | React.MouseEvent) => {
    e?.preventDefault()
    setLoading(true)
    setError(null)
    setMessage(null)
    clearPoll()
    const corrId = newUUID()
    const idempotencyKey = newUUID()
    try {
      const res = await postAuditZip(form, corrId, cfg.tenantId, idempotencyKey)
      const payload = await safeJson(res)
      if (!res.ok) {
        setError(handleErrorResponse(res.status, payload, res.headers.get('Retry-After') || undefined))
        return
      }
      const data = payload as AuditZipJob
      setJob(data)
      setMessage('ジョブを受け付けました')
      pollJob(data.jobId)
    } catch (err: any) {
      enqueueOffline({ idempotencyKey, payload: form, corrId, tenantId: cfg.tenantId, attempts: 0, nextAttemptAt: Date.now() })
      setMessage('オフラインのため再送キューに追加しました')
    } finally {
      setLoading(false)
    }
  }

  const statusBadge = useMemo(() => {
    if (!job) return null
    const color: Record<AuditZipStatus, string> = {
      queued: '#fbbf24',
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

  const progress = job?.progress ?? 0

  return (
    <div style={{ ...fontStyle, background: 'radial-gradient(circle at 18% 20%, #0ea5e9 0, transparent 22%), linear-gradient(135deg,#020617,#0b1224)', minHeight: '100vh', padding: '2rem' }}>
      <main aria-labelledby="title" style={{ maxWidth: 980, margin: '0 auto', color: '#e5e7eb', display: 'grid', gap: '1rem' }}>
        <header>
          <p style={{ color: '#38bdf8', letterSpacing: '.08em', fontSize: '.85rem', margin: 0 }}>Audit ZIP</p>
          <h1 id="title" style={{ margin: 0, fontSize: '1.9rem' }}>電帳法監査ZIP生成</h1>
          <p style={{ color: '#94a3b8' }}>受付 p95 10s / バックグラウンド p95 60s / 署名URL TTL 10分 / 保管 7日。</p>
        </header>

        <section aria-label="ステータス" style={cardStyle}>
          {message && <div role="status" style={infoStyle}>{message}</div>}
          {error && <div role="alert" style={errorStyle}>{error}</div>}
          {!!queued.length && (
            <div role="status" style={queueStyle}>
              <span>再送待ち {queued.length} 件</span>
              <div style={{ display: 'flex', gap: '.5rem', alignItems: 'center' }}>
                <small>次の再送: {formatNextRetry(queued)}</small>
                <button onClick={() => processQueue()} style={pillBtnStyle}>今すぐ再送</button>
              </div>
            </div>
          )}
          {job && (
            <div style={{ marginTop: '.5rem', display: 'flex', gap: '1rem', alignItems: 'center', flexWrap: 'wrap' }}>
              <div>Job ID: <code>{job.jobId}</code></div>
              {statusBadge}
              {job.result?.signedUrl && (
                <a href={job.result.signedUrl} style={linkStyle} target="_blank" rel="noreferrer">
                  署名URL ({(job.result.size / 1024).toFixed(1)} KB, {formatDateTime(job.result.expiresAt)})
                </a>
              )}
            </div>
          )}
        </section>

        <form onSubmit={handleSubmit} style={cardStyle}>
          <fieldset style={{ border: 'none', padding: 0, margin: 0 }}>
            <legend style={legendStyle}>抽出条件</legend>
            <div style={gridCols(180)}>
              <LabeledInput label="From" type="date" value={form.from} onChange={(v) => setForm({ ...form, from: v })} />
              <LabeledInput label="To" type="date" value={form.to} onChange={(v) => setForm({ ...form, to: v })} />
              <LabeledInput label="取引先コード (任意)" value={form.partner || ''} onChange={(v) => setForm({ ...form, partner: v || null })} placeholder="BRV-123" />
            </div>
            <div style={gridCols(200)}>
              <LabeledInput label="金額下限 (円)" type="number" value={form.minAmount ?? ''} onChange={(v) => setForm({ ...form, minAmount: v ? Number(v) : null })} />
              <LabeledInput label="金額上限 (円)" type="number" value={form.maxAmount ?? ''} onChange={(v) => setForm({ ...form, maxAmount: v ? Number(v) : null })} />
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
            <span aria-live="polite" style={{ color: '#94a3b8' }}>Tab/Shift+Tab で移動、Enter で送信。オフライン時はキューに退避します。</span>
          </div>
        </form>

        {job && (
          <section aria-label="進捗" style={cardStyle}>
            <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', gap: '0.5rem', flexWrap: 'wrap' }}>
              <div>
                <p style={{ margin: 0, color: '#cbd5e1' }}>進捗</p>
                <div style={{ width: '100%', background: '#0f172a', borderRadius: 10, border: '1px solid #1f2937', height: 12 }}>
                  <div style={{ width: `${progress}%`, height: '100%', background: '#22c55e', borderRadius: 10, transition: 'width .3s ease' }} aria-valuenow={progress} aria-valuemin={0} aria-valuemax={100} role="progressbar" />
                </div>
              </div>
              <div style={{ display: 'flex', gap: '.5rem', flexWrap: 'wrap' }}>
                {job.status === 'running' && job.canCancel !== false && (
                  <button onClick={cancelJob} disabled={loading || job.status !== 'running'} style={{ ...pillBtnStyle, background: '#f97316', color: '#0b1221' }}>
                    キャンセル
                  </button>
                )}
                {(job.status === 'failed' || job.status === 'canceled') && (
                  <button onClick={(e) => handleSubmit(e)} disabled={loading} style={{ ...pillBtnStyle, background: '#38bdf8', color: '#0b1221' }}>
                    再実行
                  </button>
                )}
              </div>
            </div>
            <div style={{ marginTop: '.5rem', color: '#94a3b8', display: 'grid', gap: '0.25rem', gridTemplateColumns: 'repeat(auto-fit,minmax(200px,1fr))' }}>
              <div>開始: {job.startedAt ? formatDateTime(job.startedAt) : '未開始'}</div>
              <div>完了: {job.finishedAt ? formatDateTime(job.finishedAt) : '未完了'}</div>
              <div>リトライ: {job.retryCount}/3</div>
            </div>
            {job.error?.message && <div style={errorStyle}>エラー: {job.error.message}</div>}
          </section>
        )}
      </main>
    </div>
  )
}

function LabeledInput(props: { label: string; value: string | number; onChange?: (v: string) => void; type?: string; placeholder?: string }) {
  const { label, value, onChange, type = 'text', placeholder } = props
  const id = useId()
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

function gridCols(minWidth: number): React.CSSProperties {
  return { display: 'grid', gridTemplateColumns: `repeat(auto-fit, minmax(${minWidth}px, 1fr))`, gap: '0.75rem' }
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
const fontStyle = { fontFamily: '"IBM Plex Sans JP", "Inter", "Noto Sans JP", "Hiragino Sans", system-ui, sans-serif' }

async function postAuditZip(body: AuditZipRequest, corrId: string, tenantId: string, idempotencyKey: string) {
  return fetch(`${cfg.apiBase}/audit/zip`, {
    method: 'POST',
    headers: {
      'Content-Type': 'application/json',
      'X-Correlation-Id': corrId,
      'X-Tenant-Id': tenantId,
      'Idempotency-Key': idempotencyKey,
    },
    body: JSON.stringify(body),
  })
}

function newUUID() {
  return typeof crypto !== 'undefined' && crypto.randomUUID ? crypto.randomUUID() : `${Date.now()}-${Math.random().toString(16).slice(2)}`
}

function formatDateTime(input?: string | null) {
  if (!input) return ''
  return new Intl.DateTimeFormat('ja-JP', { dateStyle: 'medium', timeStyle: 'short', timeZone: 'Asia/Tokyo' }).format(new Date(input))
}

function formatNextRetry(queue: QueueItem[]) {
  const next = Math.min(...queue.map((q) => q.nextAttemptAt))
  if (!Number.isFinite(next)) return '未定'
  return new Intl.DateTimeFormat('ja-JP', { timeStyle: 'medium', timeZone: 'Asia/Tokyo' }).format(new Date(next))
}

function toISO(d: Date) {
  return d.toISOString().slice(0, 10)
}

async function safeJson(res: Response) {
  try {
    return await res.json()
  } catch {
    return null
  }
}

function handleErrorResponse(status: number, payload: unknown, retryAfter?: string) {
  if (status === 400 && payload && typeof payload === 'object' && 'errors' in (payload as any)) {
    const v = payload as ValidationError
    return v.errors.map((e) => `${e.path}: ${e.message}`).join(' / ') || v.message
  }
  if (status === 413 && payload && typeof payload === 'object' && 'splitHint' in (payload as any)) {
    const tooLarge = payload as RequestTooLargeError
    return `対象が大きすぎます。推奨分割: ${tooLarge.splitHint.chunks} チャンク / 約 ${tooLarge.splitHint.approxSizeMB} MB`
  }
  if (status === 409 && payload && typeof payload === 'object' && 'conflictReason' in (payload as any)) {
    const conflict = payload as ConflictError
    return `409: ${conflict.conflictReason}`
  }
  if (status === 429) {
    return `429: レート制限中です。Retry-After: ${retryAfter ?? '確認してください'}`
  }
  return 'リクエストに失敗しました'
}
