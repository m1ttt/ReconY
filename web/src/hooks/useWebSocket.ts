import { useEffect, useRef, useCallback } from 'react'
import { useStore } from '../store'

export function useWebSocket(workspaceId?: string) {
  const wsRef = useRef<WebSocket | null>(null)
  const addEvent = useStore((s) => s.addEvent)

  const connect = useCallback((onReconnect?: () => void) => {
    const protocol = location.protocol === 'https:' ? 'wss:' : 'ws:'
    const wsUrl = `${protocol}//${location.host}/api/v1/ws${workspaceId ? `?workspace_id=${workspaceId}` : ''}`

    const ws = new WebSocket(wsUrl)
    wsRef.current = ws

    ws.onmessage = (e) => {
      try {
        const event = JSON.parse(e.data)
        addEvent(event)
      } catch {}
    }

    ws.onclose = () => {
      if (onReconnect) onReconnect()
    }

    return ws
  }, [workspaceId, addEvent])

  useEffect(() => {
    let mounted = true
    let reconnectTimer: ReturnType<typeof setTimeout> | null = null

    const scheduleReconnect = () => {
      if (!mounted || reconnectTimer) return
      reconnectTimer = setTimeout(() => {
        reconnectTimer = null
        if (!mounted) return
        connect(scheduleReconnect)
      }, 3000)
    }

    const ws = connect(scheduleReconnect)

    return () => {
      mounted = false
      if (reconnectTimer) {
        clearTimeout(reconnectTimer)
        reconnectTimer = null
      }
      if (wsRef.current) {
        wsRef.current.onclose = null
        wsRef.current.close()
        wsRef.current = null
      } else {
        ws.onclose = null
        ws.close()
      }
    }
  }, [connect])
}
