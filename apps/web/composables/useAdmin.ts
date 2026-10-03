import type { RoomListItem } from '~/types/protocol'

export function useAdmin() {
  const api = useApi()
  const authenticated = useState<boolean>('admin-authenticated', () => false)
  const resolved = useState<boolean>('admin-auth-resolved', () => false)
  const rooms = useState<RoomListItem[]>('admin-rooms', () => [])
  const loading = ref(false)
  async function check() {
    try {
      await api('/api/auth/me')
      authenticated.value = true
    } catch {
      authenticated.value = false
    } finally {
      resolved.value = true
    }
    return authenticated.value
  }
  async function login(email: string, password: string) { await api('/api/auth/login', { method: 'POST', body: { email, password } }); authenticated.value = true; resolved.value = true }
  async function logout() { await api('/api/auth/logout', { method: 'POST' }); authenticated.value = false; resolved.value = true; rooms.value = [] }
  async function loadRooms() { loading.value = true; try { rooms.value = await api<RoomListItem[]>('/api/rooms') } finally { loading.value = false } }
  return { authenticated, resolved, rooms, loading, check, login, logout, loadRooms }
}
