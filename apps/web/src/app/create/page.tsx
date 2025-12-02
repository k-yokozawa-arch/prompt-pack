'use client'

import { useEffect } from 'react'
import { useRouter } from 'next/navigation'

export default function RedirectCreate() {
  const router = useRouter()
  useEffect(() => {
    router.replace('/invoice/create')
  }, [router])
  return null
}
