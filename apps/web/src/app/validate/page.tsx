'use client'

import { useEffect } from 'react'
import { useRouter } from 'next/navigation'

export default function RedirectValidate() {
  const router = useRouter()
  useEffect(() => {
    router.replace('/invoice/validate')
  }, [router])
  return null
}
