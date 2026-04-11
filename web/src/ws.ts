import { useEffect, useRef } from 'react'
import { useSimStore } from './store'
import type { WSMessage } from './types'

export function useWebSocket(deviceId: string | null) {
  const setConnected = useSimStore((s) => s.setConnected)
  const updateValues = useSimStore((s) => s.updateValues)
  const reconnectTimer = useRef<ReturnType<typeof setTimeout> | null>(null)
  const wsRef = useRef<WebSocket | null>(null)

  useEffect(() => {
    if (!deviceId) {
      setConnected(false)
      return
    }

    let alive = true

    function connect() {
      if (!alive) return
      const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:'
      const ws = new WebSocket(`${protocol}//${window.location.host}/ws?device=${deviceId}`)
      wsRef.current = ws

      ws.onopen = () => {
        if (!alive) { ws.close(); return }
        setConnected(true)
        if (reconnectTimer.current) {
          clearTimeout(reconnectTimer.current)
          reconnectTimer.current = null
        }
      }

      ws.onmessage = (ev) => {
        if (!alive) return
        try {
          const msg: WSMessage = JSON.parse(ev.data as string)
          if (msg.type === 'snapshot') {
            updateValues(msg.registers)
          }
        } catch {
          // ignore parse errors
        }
      }

      ws.onclose = () => {
        setConnected(false)
        if (alive) {
          reconnectTimer.current = setTimeout(connect, 2000)
        }
      }

      ws.onerror = () => {
        ws.close()
      }
    }

    connect()

    return () => {
      alive = false
      if (reconnectTimer.current) {
        clearTimeout(reconnectTimer.current)
        reconnectTimer.current = null
      }
      wsRef.current?.close()
      wsRef.current = null
    }
  }, [deviceId, setConnected, updateValues])

  return { connected: useSimStore((s) => s.connected) }
}
