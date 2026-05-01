import { useEffect, useRef, useCallback, useState } from 'react'

export interface WSMessage {
  type: string
  level?: string
  message?: string
  tool?: string
  result?: string
  content?: string
  flag?: string
  success?: boolean
  error?: string
  iterations?: number
  duration?: string
}

export function useWebSocket(onMessage: (msg: WSMessage) => void) {
  const wsRef = useRef<WebSocket | null>(null)
  const onMessageRef = useRef(onMessage)
  onMessageRef.current = onMessage
  const [connected, setConnected] = useState(false)

  const connect = useCallback(() => {
    const protocol = location.protocol === 'https:' ? 'wss:' : 'ws:'
    const ws = new WebSocket(`${protocol}//${location.host}/ws`)
    wsRef.current = ws

    ws.onopen = () => setConnected(true)
    ws.onclose = () => {
      setConnected(false)
      setTimeout(connect, 3000)
    }
    ws.onmessage = (e) => {
      try {
        onMessageRef.current(JSON.parse(e.data))
      } catch {}
    }
  }, [])

  useEffect(() => {
    connect()
    return () => {
      wsRef.current?.close()
    }
  }, [connect])

  return { connected }
}
