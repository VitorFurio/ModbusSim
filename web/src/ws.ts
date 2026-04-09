import { useEffect, useRef } from 'react'
import { useSimStore } from './store'
import type { WSMessage } from './types'

export function useWebSocket() {
  const setConnected = useSimStore((s) => s.setConnected)
  const updateValues = useSimStore((s) => s.updateValues)
  const reconnectTimer = useRef<ReturnType<typeof setTimeout> | null>(null)
  const wsRef = useRef<WebSocket | null>(null)

  useEffect(() => {
    function connect() {
      const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:'
      const ws = new WebSocket(`${protocol}//${window.location.host}/ws`)
      wsRef.current = ws

      ws.onopen = () => {
        setConnected(true)
        if (reconnectTimer.current) {
          clearTimeout(reconnectTimer.current)
          reconnectTimer.current = null
        }
      }

      ws.onmessage = (ev) => {
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
        reconnectTimer.current = setTimeout(connect, 2000)
      }

      ws.onerror = () => {
        ws.close()
      }
    }

    connect()

    return () => {
      if (reconnectTimer.current) clearTimeout(reconnectTimer.current)
      wsRef.current?.close()
    }
  }, [setConnected, updateValues])

  return { connected: useSimStore((s) => s.connected) }
}
