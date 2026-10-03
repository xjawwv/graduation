import type { ClientMessage, ConnectionState, JoinMessage, ServerMessage } from '~/types/protocol'

export function useWebSocket(join: () => JoinMessage | null) {
  const config = useRuntimeConfig()
  const state = ref<ConnectionState>('offline')
  const latency = ref<number | null>(null)
  const lastMessage = shallowRef<ServerMessage | null>(null)
  const listeners = new Set<(message: ServerMessage) => void>()
  let socket: WebSocket | null = null
  let retry = 0
  let retryTimer: number | undefined
  let heartbeat: number | undefined
  let manuallyClosed = false
  const handleOffline = () => {
    state.value = 'offline'
    socket?.close()
  }

  function emit(message: ServerMessage) {
    lastMessage.value = message
    if (message.type === 'pong' && message.sent_at) latency.value = Math.max(0, Date.now() - message.sent_at)
    for (const listener of listeners) listener(message)
  }
  function connect() {
    if (!import.meta.client || socket?.readyState === WebSocket.OPEN || socket?.readyState === WebSocket.CONNECTING) return
    const payload = join()
    if (!payload) return
    manuallyClosed = false
    state.value = retry ? 'reconnecting' : 'connecting'
    const current = new WebSocket(config.public.wsUrl)
    socket = current
    current.addEventListener('open', () => {
      if (socket !== current) return
      retry = 0
      state.value = 'connected'
      current.send(JSON.stringify(payload))
      clearInterval(heartbeat)
      heartbeat = window.setInterval(() => send({ type: 'ping', sent_at: Date.now() }), 15_000)
    })
    current.addEventListener('message', (event) => {
      if (socket !== current) return
      try { emit(JSON.parse(String(event.data)) as ServerMessage) } catch { /* malformed server data is ignored */ }
    })
    current.addEventListener('close', () => {
      if (socket !== current) return
      socket = null
      clearInterval(heartbeat)
      heartbeat = undefined
      if (manuallyClosed) { state.value = 'offline'; return }
      if (!navigator.onLine) {
        state.value = 'offline'
        return
      }
      state.value = 'reconnecting'
      const delays = [500, 1000, 2000, 4000, 5000]
      const delay = delays[Math.min(retry, delays.length - 1)]
      retry += 1
      retryTimer = window.setTimeout(connect, delay)
    })
    current.addEventListener('error', () => current.close())
  }
  function send(message: ClientMessage): boolean {
    if (socket?.readyState !== WebSocket.OPEN) return false
    socket.send(JSON.stringify(message))
    return true
  }
  function onMessage(listener: (message: ServerMessage) => void) { listeners.add(listener); return () => listeners.delete(listener) }
  function close() {
    manuallyClosed = true
    clearTimeout(retryTimer)
    clearInterval(heartbeat)
    retryTimer = undefined
    heartbeat = undefined
    const current = socket
    socket = null
    current?.close(1000, 'page closed')
    state.value = 'offline'
  }
  if (import.meta.client) {
    window.addEventListener('online', connect)
    window.addEventListener('offline', handleOffline)
  }
  onBeforeUnmount(() => {
    if (import.meta.client) {
      window.removeEventListener('online', connect)
      window.removeEventListener('offline', handleOffline)
    }
    close()
  })
  return { state: readonly(state), latency: readonly(latency), lastMessage: readonly(lastMessage), connect, send, onMessage, close }
}
