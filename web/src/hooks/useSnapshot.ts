import { useState, useEffect, useRef, useCallback } from 'react'
import type { Snapshot, PublicConfig, ConnectionState } from '../types'
import { fetchPublicConfig } from '../api/config'

interface UseSnapshotReturn {
  snapshot: Snapshot | null
  connectionState: ConnectionState
  errorMessage: string
  publicConfig: PublicConfig
}

const defaultPublicConfig: PublicConfig = {
  refresh_interval: '5s',
  refresh_interval_seconds: 5,
  show_query_clients: false,
  server_host: '',
  server_port: 0,
}

export function useSnapshot(): UseSnapshotReturn {
  const [snapshot, setSnapshot] = useState<Snapshot | null>(null)
  const [publicConfig, setPublicConfig] = useState<PublicConfig>(defaultPublicConfig)
  const [connectionState, setConnectionState] = useState<ConnectionState>('connecting')
  const [errorMessage, setErrorMessage] = useState('')

  const closedRef = useRef(false)
  const fallbackTimerRef = useRef<number | null>(null)
  const eventSourceRef = useRef<EventSource | null>(null)

  const stopFallback = useCallback(() => {
    if (fallbackTimerRef.current !== null) {
      window.clearInterval(fallbackTimerRef.current)
      fallbackTimerRef.current = null
    }
  }, [])

  const fetchSnapshot = useCallback(async () => {
    try {
      const response = await fetch('/api/v1/snapshot', { cache: 'no-store' })
      const payload = await response.json()
      if (closedRef.current) return

      if (!response.ok) {
        setConnectionState('waiting')
        setErrorMessage(payload.error || 'snapshot_not_ready')
        return
      }

      setSnapshot(payload)
      setConnectionState(payload.meta?.stale ? 'stale' : 'live')
      setErrorMessage(payload.meta?.last_error || '')
    } catch {
      if (!closedRef.current) {
        setConnectionState('waiting')
      }
    }
  }, [])

  const scheduleFallback = useCallback(
    (intervalMs: number) => {
      stopFallback()
      fallbackTimerRef.current = window.setInterval(fetchSnapshot, intervalMs)
    },
    [stopFallback, fetchSnapshot],
  )

  useEffect(() => {
    closedRef.current = false

    // 获取公开配置
    fetchPublicConfig()
      .then((cfg) => {
        if (!closedRef.current) setPublicConfig(cfg)
        scheduleFallback(Math.max(3000, (cfg.refresh_interval_seconds || 5) * 1000))
      })
      .catch(() => {
        scheduleFallback(5000)
      })
      .finally(fetchSnapshot)

    // SSE 连接
    if (typeof EventSource !== 'undefined') {
      const es = new EventSource('/api/v1/stream')
      eventSourceRef.current = es

      es.addEventListener('snapshot', (event) => {
        if (closedRef.current) return
        const payload: Snapshot = JSON.parse(event.data)
        setSnapshot(payload)
        setConnectionState(payload.meta?.stale ? 'stale' : 'live')
        setErrorMessage(payload.meta?.last_error || '')
        stopFallback()
      })

      es.onerror = () => {
        if (closedRef.current) return
        setConnectionState('waiting')
        scheduleFallback(
          Math.max(3000, (publicConfig.refresh_interval_seconds || 5) * 1000),
        )
      }
    }

    return () => {
      closedRef.current = true
      stopFallback()
      if (eventSourceRef.current) {
        eventSourceRef.current.close()
        eventSourceRef.current = null
      }
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [])

  return { snapshot, connectionState, errorMessage, publicConfig }
}
