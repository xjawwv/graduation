export type Role = 'student' | 'display' | 'admin'
export type RoomStatus = 'WAITING' | 'ACTIVE' | 'PAUSED' | 'ENDED'
export type DisplayMode = 'REACTIONS' | 'ANNOUNCEMENT' | 'COUNTDOWN' | 'CELEBRATION' | 'QR' | 'BLANK'
export type ConnectionState = 'connecting' | 'connected' | 'reconnecting' | 'offline'

export interface Presence { online: number; students: number; admins: number; displays: number; unique_clients: number }
export interface RoomState { code: string; title: string; status: RoomStatus; locked: boolean; background?: string }
export interface DisplayHealth { id: string; name: string; online: boolean; latency_ms: number; last_heartbeat: number }
export interface JoinMessage { type: 'join'; role: Role; room: string; client_id?: string; token?: string; name?: string }
export type ClientMessage = JoinMessage | { type: 'reaction'; emoji: string } | { type: 'ping'; sent_at: number } | AdminMessage
export type AdminMessage =
  | { type: 'pause' | 'resume' | 'clear_display' | 'lock_room' | 'unlock_room' | 'disconnect_all' }
  | { type: 'set_display_mode'; mode: DisplayMode; duration?: number }
  | { type: 'set_display_background'; background: string }
  | { type: 'announcement'; message: string; duration: number }
  | { type: 'prompt'; message: string }
  | { type: 'reaction_config'; reactions: string[] }
export interface ServerMessage {
  type: 'joined' | 'reaction_batch' | 'presence' | 'room_state' | 'reaction_config' | 'display_mode' | 'announcement' | 'prompt' | 'clear_display' | 'display_health' | 'system' | 'pong'
  message?: string
  room?: RoomState
  presence?: Presence
  reactions?: Record<string, number>
  reaction_config?: string[]
  mode?: DisplayMode
  background?: string
  duration?: number
  countdown_ends_at?: number
  displays?: DisplayHealth[]
  server_time?: number
  sent_at?: number
  code?: string
}

export interface RoomRecord {
  id: number; code: string; name: string; title: string; status: RoomStatus; locked: boolean
  created_by: number; created_at: string; started_at?: string; ended_at?: string
  reactions: string[]; reaction_rate_limit: number; display_mode: DisplayMode; display_background: string
}
export interface RuntimeSnapshot {
  room: RoomRecord; presence: Presence; total_reactions: number; reaction_counts: Record<string, number>
  reactions_per_second: number; peak_reactions_per_second: number; peak_concurrent: number; unique_users: number
  rate_limited: number; displays: DisplayHealth[]; announcement: string; prompt: string; countdown_ends_at: number
}
export type RoomListItem = RoomRecord & { runtime?: RuntimeSnapshot }
export interface StatPoint { at: string; emoji: string; count: number }
export interface SystemEvent { id: number; room_id?: number; level: string; type: string; message: string; created_at: string }
export interface DisplayAuthorization { id: number; name: string; revoked: boolean; last_seen_at?: string; created_at: string }
