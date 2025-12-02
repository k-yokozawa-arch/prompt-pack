import type { Metadata } from 'next'

export const metadata: Metadata = {
  title: 'Audit ZIP Export',
  description: '電帳法監査ZIP生成',
}

export default function RootLayout({ children }: { children: React.ReactNode }) {
  return (
    <html lang="ja">
      <body style={{ margin: 0, background: '#020617' }}>{children}</body>
    </html>
  )
}
