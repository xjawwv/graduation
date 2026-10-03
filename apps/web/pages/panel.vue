<script setup lang="ts">
import {
  Activity,
  BarChart3,
  CircleDot,
  Copy,
  DoorOpen,
  GraduationCap,
  KeyRound,
  LayoutDashboard,
  Link2,
  LockKeyhole,
  LogIn,
  LogOut,
  Plus,
  QrCode,
  RadioTower,
  Save,
  Settings2,
  ShieldCheck,
  Sparkles,
  Trash2,
  Users,
  Wifi,
  WifiOff,
  X,
  Zap,
} from '@lucide/vue'
import QrcodeVue from 'qrcode.vue'
import type { AdminMessage, DisplayAuthorization, DisplayHealth, DisplayMode, RuntimeSnapshot, StatPoint, SystemEvent } from '~/types/protocol'

const api = useApi()
const admin = useAdmin()
const email = ref('')
const password = ref('')
const authError = ref('')
type PanelSection = 'Dashboard' | 'Live Control' | 'Rooms' | 'Analytics' | 'System'
const nav = ref<PanelSection>('Dashboard')
const selectedCode = ref('')
const displays = ref<DisplayHealth[]>([])
const livePresence = ref({ online: 0, students: 0, admins: 0, displays: 0, unique_clients: 0 })
const liveTotal = ref(0)
const liveRate = ref(0)
const systemMetrics = ref<Record<string, string | number>>({})
const systemEvents = ref<SystemEvent[]>([])
const analytics = ref<{ runtime?: RuntimeSnapshot; series: StatPoint[] }>({ series: [] })
const showCreate = ref(false)
const createForm = reactive({ code: '', name: '', title: '' })
const displayName = ref('Main Videotron')
const displayLink = ref('')
const displayAuthorizations = ref<DisplayAuthorization[]>([])
const qrFullscreen = ref(false)
const error = ref('')
const deletingRoom = ref(false)
let systemTimer: number | undefined
let liveRateTimer: number | undefined
const selected = computed(() => admin.rooms.value.find((item) => item.code === selectedCode.value))
const joinURL = computed(() => import.meta.client && selected.value ? `${location.origin}/join/${selected.value.code}` : '')
const timeline = computed(() => {
  const buckets: Record<string, number> = {}
  for (const point of analytics.value.series) buckets[point.at] = (buckets[point.at] || 0) + point.count
  return Object.entries(buckets).sort(([left], [right]) => left.localeCompare(right)).slice(-60).map(([at, count]) => ({ at, count }))
})
const timelineMax = computed(() => Math.max(1, ...timeline.value.map((point) => point.count)))
const sessionDuration = computed(() => {
  const runtimeRoom = analytics.value.runtime?.room
  if (!runtimeRoom?.started_at) return 'Not started'
  const elapsed = Math.max(0, new Date(runtimeRoom.ended_at || Date.now()).getTime() - new Date(runtimeRoom.started_at).getTime())
  const minutes = Math.floor(elapsed / 60_000)
  return `${Math.floor(minutes / 60)}h ${minutes % 60}m`
})
const adminWS = useWebSocket(() => selectedCode.value ? { type: 'join', role: 'admin', room: selectedCode.value } : null)
const joinedAdminRoom = ref('')
const controlsReady = computed(() => adminWS.state.value === 'connected' && joinedAdminRoom.value === selectedCode.value)
const tabs = [
  { label: 'Dashboard' as PanelSection, icon: LayoutDashboard },
  { label: 'Live Control' as PanelSection, icon: RadioTower },
  { label: 'Rooms' as PanelSection, icon: DoorOpen },
  { label: 'Analytics' as PanelSection, icon: BarChart3 },
  { label: 'System' as PanelSection, icon: Activity },
]
const roomStatusLabel = computed(() => selected.value ? ({ WAITING: 'Waiting', ACTIVE: 'Live', PAUSED: 'Paused', ENDED: 'Ended' })[selected.value.status] : 'No room')
const connectionLabel = computed(() => ({ connecting: 'Connecting…', connected: 'Connected', reconnecting: 'Reconnecting…', offline: 'Offline' })[adminWS.state.value])
const numberFormatter = new Intl.NumberFormat()
const dateTimeFormatter = new Intl.DateTimeFormat(undefined, { dateStyle: 'medium', timeStyle: 'short' })
const chartTimeFormatter = new Intl.DateTimeFormat(undefined, { hour: '2-digit', minute: '2-digit', second: '2-digit' })
function formatNumber(value: number) { return numberFormatter.format(value) }
function formatDateTime(value: string) { return dateTimeFormatter.format(new Date(value)) }
function formatChartTime(value: string) { return chartTimeFormatter.format(new Date(value)) }

async function authenticate() {
  authError.value = ''
  try { await admin.login(email.value, password.value); password.value = ''; await initialize() }
  catch { authError.value = 'Invalid email or password.' }
}
async function initialize() {
  await admin.loadRooms()
  if (!selectedCode.value && admin.rooms.value.length) selectedCode.value = admin.rooms.value[0].code
  connectAdmin()
}
function connectAdmin() {
  joinedAdminRoom.value = ''
  adminWS.close()
  const expectedCode = selectedCode.value
  nextTick(() => { if (selectedCode.value === expectedCode) adminWS.connect() })
}
async function refreshRooms() { await admin.loadRooms(); if (!admin.rooms.value.some((r) => r.code === selectedCode.value)) selectedCode.value = admin.rooms.value[0]?.code || '' }
async function createRoom() {
  error.value = ''
  try { const created = await api<{ code: string }>('/api/rooms', { method: 'POST', body: createForm }); showCreate.value = false; Object.assign(createForm, { code: '', name: '', title: '' }); await refreshRooms(); selectedCode.value = created.code }
  catch { error.value = 'Could not create room. Check that the code is unique.' }
}
async function lifecycle(action: string, code: string) { await api(`/api/rooms/${code}/${action}`, { method: 'POST' }); await refreshRooms() }
async function command(type: string, payload: Record<string, unknown> = {}) {
  const room = selected.value
  if (!room) return
  const targetCode = room.code
  if (!controlsReady.value) {
    error.value = `Controls are still connecting to ${targetCode}. Try again in a moment.`
    return
  }
  error.value = ''
  if (type === 'end') { if (!confirm(`End ${targetCode}? Students can no longer react.`)) return; await lifecycle('end', targetCode); return }
  if (type === 'disconnect_all' && !confirm(`Disconnect every student and display from ${targetCode}?`)) return
  if (['pause', 'resume', 'lock_room', 'unlock_room'].includes(type)) {
    const endpoint: Record<string, string> = { pause: 'pause', resume: room.status === 'WAITING' ? 'start' : 'resume', lock_room: 'lock', unlock_room: 'unlock' }
    await lifecycle(endpoint[type], targetCode)
    return
  }
  if (type === 'set_display_background') {
    const background = String(payload.background || '')
    await api(`/api/rooms/${targetCode}`, { method: 'PATCH', body: { display_background: background } })
    if (selected.value?.code === targetCode) selected.value.display_background = background
    return
  }
  if (!adminWS.send({ type, ...payload } as AdminMessage)) {
    error.value = `Controls lost their connection to ${targetCode}. Wait for reconnection and try again.`
    return
  }
  if (type === 'set_display_mode' && selected.value?.code === targetCode) selected.value.display_mode = payload.mode as DisplayMode
}
async function saveReactions(reactions: string[]) { const room = selected.value; if (!room) return; await api(`/api/rooms/${room.code}`, { method: 'PATCH', body: { reactions } }); if (selected.value?.code === room.code) selected.value.reactions = reactions }
async function updateRoom() { if (!selected.value) return; await api(`/api/rooms/${selected.value.code}`, { method: 'PATCH', body: { name: selected.value.name, title: selected.value.title } }); await refreshRooms() }
async function deleteRoom() {
  const room = selected.value
  if (!room || deletingRoom.value || !confirm(`Delete ${room.code}? This disconnects everyone in the room and cannot be undone.`)) return
  error.value = ''
  deletingRoom.value = true
  try {
    await api(`/api/rooms/${room.code}`, { method: 'DELETE' })
    selectedCode.value = ''
    await refreshRooms()
  } catch {
    error.value = 'Room could not be deleted. Refresh the room list and try again.'
  } finally {
    deletingRoom.value = false
  }
}
async function loadDisplayAuthorizations() { if (selected.value) displayAuthorizations.value = await api(`/api/rooms/${selected.value.code}/displays`) }
async function authorizeDisplay() { if (!selected.value) return; const result = await api<{ token: string }>(`/api/rooms/${selected.value.code}/displays`, { method: 'POST', body: { name: displayName.value } }); displayLink.value = `${location.origin}/display/${selected.value.code}?token=${encodeURIComponent(result.token)}`; await loadDisplayAuthorizations() }
async function copyDisplayLink() { if (displayLink.value) await navigator.clipboard.writeText(displayLink.value) }
async function revokeDisplay(id: number, name: string) { if (!selected.value || !confirm(`Revoke ${name}? An active display will disconnect.`)) return; await api(`/api/rooms/${selected.value.code}/displays/${id}`, { method: 'DELETE' }); await loadDisplayAuthorizations() }
async function loadAnalytics() { if (selected.value) analytics.value = await api(`/api/rooms/${selected.value.code}/stats`) }
async function loadSystem() { const result = await api<{ metrics: Record<string, string | number>; events: SystemEvent[] }>('/api/system'); systemMetrics.value = result.metrics; systemEvents.value = result.events }
async function logout() { adminWS.close(); await admin.logout() }

watch(selectedCode, () => { joinedAdminRoom.value = ''; displays.value = []; displayLink.value = ''; connectAdmin(); loadDisplayAuthorizations(); if (nav.value === 'Analytics') loadAnalytics() })
watch(nav, (value) => { if (value === 'Analytics') loadAnalytics(); if (value === 'System') loadSystem() })
onMounted(async () => {
  adminWS.onMessage((message) => {
    if (message.type === 'joined' && message.room) {
      if (message.room.code !== selectedCode.value) return
      joinedAdminRoom.value = message.room.code
    }
    if (joinedAdminRoom.value !== selectedCode.value) return
    if (message.presence) livePresence.value = message.presence
    if (message.displays) displays.value = message.displays
    if (message.room && selected.value) { selected.value.status = message.room.status; selected.value.locked = message.room.locked; selected.value.title = message.room.title; if (message.room.background) selected.value.display_background = message.room.background }
    if (message.reactions) {
      const count = Object.values(message.reactions).reduce((total, value) => total + value, 0)
      liveTotal.value += count
      liveRate.value = Math.round(count / .15)
      clearTimeout(liveRateTimer)
      liveRateTimer = window.setTimeout(() => { liveRate.value = 0 }, 1_000)
    }
    if (message.reaction_config && selected.value) selected.value.reactions = message.reaction_config
    if (message.mode && selected.value) selected.value.display_mode = message.mode
    if (message.background && selected.value) selected.value.display_background = message.background
  })
  if (await admin.check()) await initialize()
  systemTimer = window.setInterval(() => { if (nav.value === 'System') loadSystem() }, 5000)
})
onBeforeUnmount(() => {
  clearInterval(systemTimer)
  clearTimeout(liveRateTimer)
})
useHead({ title: 'Control Room · Graduation Live' })
</script>

<template>
  <main v-if="!admin.resolved.value" class="grid min-h-dvh place-items-center bg-[#0d1014] p-5" aria-live="polite">
    <div class="border-l-2 border-[#d6a84b] bg-[#13181e] px-7 py-6">
      <div class="flex items-center gap-3">
        <GraduationCap :size="21" class="text-[#d6a84b]" aria-hidden="true" />
        <div><p class="font-semibold text-[#f2eee3]">Graduation Control Room</p><p class="mt-1 text-sm text-[#89929d]">Restoring secure session…</p></div>
      </div>
    </div>
  </main>
  <main v-else-if="!admin.authenticated.value" class="grid min-h-dvh place-items-center bg-[#0d1014] p-5">
    <form class="w-full max-w-md border-l-2 border-[#d6a84b] bg-[#13181e] p-7 shadow-2xl shadow-black/30" @submit.prevent="authenticate">
      <div class="mb-8 flex items-start gap-4">
        <span class="grid h-11 w-11 shrink-0 place-items-center border border-[#3a424c] bg-[#1b2128] text-[#d6a84b]"><GraduationCap :size="23" aria-hidden="true" /></span>
        <div>
          <h1 class="text-2xl font-semibold tracking-tight text-[#f2eee3] text-balance">Graduation Control Room</h1>
          <p class="mt-1 text-sm text-[#89929d]">Sign in to operate the live room.</p>
        </div>
      </div>
      <label class="label" for="admin-email">Email</label>
      <input id="admin-email" v-model="email" class="input mb-4" name="email" type="email" autocomplete="username" spellcheck="false" required>
      <label class="label" for="admin-password">Password</label>
      <input id="admin-password" v-model="password" class="input" name="password" type="password" autocomplete="current-password" required minlength="12">
      <p v-if="authError" class="mt-3 text-sm text-[#ef9b92]" aria-live="polite">{{ authError }} Check your credentials and try again.</p>
      <button class="btn-primary mt-6 w-full" type="submit"><LogIn :size="17" aria-hidden="true" /> Sign In</button>
    </form>
  </main>

  <div v-else class="min-h-dvh overflow-x-hidden bg-[#0d1014] lg:pl-60">
    <a href="#panel-main" class="skip-link">Skip to main content</a>
    <aside class="border-b border-[#2a3038] bg-[#10151a] lg:fixed lg:inset-y-0 lg:left-0 lg:z-40 lg:flex lg:w-60 lg:flex-col lg:overflow-y-auto lg:border-b-0 lg:border-r">
      <div class="flex items-center justify-between border-b border-[#2a3038] px-4 py-4">
        <div class="flex items-center gap-3">
          <span class="grid h-9 w-9 place-items-center bg-[#d6a84b] text-[#15110a]"><GraduationCap :size="20" aria-hidden="true" /></span>
          <div>
            <p class="font-semibold leading-tight text-[#eee9dc]">Graduation Live</p>
            <p class="text-xs text-[#858f9a]">Control Room</p>
          </div>
        </div>
      </div>

      <nav class="flex gap-1 overflow-x-auto p-3 lg:flex-col" aria-label="Admin sections">
        <button
          v-for="tab in tabs"
          :key="tab.label"
          class="inline-flex shrink-0 items-center gap-3 rounded-md px-3 py-2.5 text-left text-sm transition-colors focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-[#d6a84b]"
          :class="nav===tab.label ? 'bg-[#1d1b16] text-[#f0cc80]' : 'text-[#929ba5] hover:bg-[#171d23] hover:text-[#e2ded4]'"
          :aria-current="nav === tab.label ? 'page' : undefined"
          @click="nav=tab.label"
        >
          <component :is="tab.icon" :size="17" aria-hidden="true" />
          {{ tab.label }}
        </button>
      </nav>

      <div class="hidden border-t border-[#2a3038] p-4 lg:block">
        <label class="label" for="sidebar-room">Active Room</label>
        <select id="sidebar-room" v-model="selectedCode" class="input font-mono text-sm" name="sidebar-room">
          <option v-for="item in admin.rooms.value" :key="item.code" :value="item.code">{{ item.code }} — {{ item.name }}</option>
        </select>
        <div class="mt-3 flex items-center gap-2 text-xs" :class="adminWS.state.value === 'connected' ? 'text-[#73cfa1]' : 'text-[#d8ae55]'">
          <Wifi v-if="adminWS.state.value === 'connected'" :size="14" aria-hidden="true" />
          <WifiOff v-else :size="14" aria-hidden="true" />
          {{ connectionLabel }}
          <span v-if="adminWS.latency.value !== null" class="ml-auto font-mono tabular-nums text-[#858f9a]">{{ adminWS.latency.value }} ms</span>
        </div>
      </div>

      <button class="m-3 mt-auto hidden items-center gap-2 rounded-md px-3 py-2 text-sm text-[#89929d] transition-colors hover:bg-[#1a2027] hover:text-[#e8e3d5] focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-[#d6a84b] lg:flex" @click="logout">
        <LogOut :size="16" aria-hidden="true" /> Log Out
      </button>
    </aside>

    <main id="panel-main" class="min-w-0">
      <header class="flex min-h-24 flex-wrap items-center justify-between gap-4 border-b border-[#2a3038] px-4 py-5 sm:px-6 lg:px-8">
        <div class="min-w-0">
          <div class="mb-1 flex items-center gap-2 text-sm text-[#858f9a]">
            <span class="font-mono" translate="no">{{ selected?.code || 'NO ROOM' }}</span>
            <span aria-hidden="true">/</span>
            <span class="inline-flex items-center gap-1.5"><span class="status-dot" :class="selected?.status === 'ACTIVE' ? 'bg-[#57b88a]' : 'bg-[#78828d]'" /> {{ roomStatusLabel }}</span>
          </div>
          <h1 class="text-2xl font-semibold tracking-tight text-[#f2eee3] sm:text-3xl text-balance">{{ nav }}</h1>
        </div>
        <div class="flex items-center gap-2">
          <select v-model="selectedCode" class="input w-auto max-w-48 font-mono lg:hidden" aria-label="Active room">
            <option v-for="item in admin.rooms.value" :key="item.code" :value="item.code">{{ item.code }}</option>
          </select>
          <button v-if="nav==='Rooms'" class="btn-primary" @click="showCreate=true"><Plus :size="17" aria-hidden="true" /> Create Room</button>
          <button class="btn-secondary lg:hidden" aria-label="Log out" title="Log out" @click="logout"><LogOut :size="17" aria-hidden="true" /></button>
        </div>
      </header>

      <section class="p-4 sm:p-6 lg:p-8">
        <p v-if="error" class="mb-4 border-l-2 border-[#d66a5e] bg-[#241919] p-3 text-sm text-[#efaaa2]" aria-live="polite">{{ error }}</p>

        <template v-if="nav==='Dashboard'">
          <section class="panel-card !p-0">
            <h2 class="sr-only">Live Summary</h2>
            <div class="grid sm:grid-cols-2 xl:grid-cols-4">
              <div class="border-b border-[#2a3038] p-5 sm:border-r xl:border-b-0">
                <div class="mb-5 flex items-center justify-between text-[#8d97a2]"><span class="text-sm">Active Rooms</span><RadioTower :size="18" aria-hidden="true" /></div>
                <p class="font-mono text-4xl font-semibold tabular-nums text-[#f0eadc]">{{ admin.rooms.value.filter(r=>r.status==='ACTIVE').length }}</p>
              </div>
              <div class="border-b border-[#2a3038] p-5 xl:border-b-0 xl:border-r">
                <div class="mb-5 flex items-center justify-between text-[#8d97a2]"><span class="text-sm">Connected Students</span><Users :size="18" aria-hidden="true" /></div>
                <p class="font-mono text-4xl font-semibold tabular-nums text-[#f0eadc]">{{ formatNumber(livePresence.students) }}</p>
              </div>
              <div class="border-b border-[#2a3038] p-5 sm:border-r xl:border-b-0">
                <div class="mb-5 flex items-center justify-between text-[#8d97a2]"><span class="text-sm">Total Reactions</span><Sparkles :size="18" aria-hidden="true" /></div>
                <p class="font-mono text-4xl font-semibold tabular-nums text-[#f0eadc]">{{ formatNumber(liveTotal || selected?.runtime?.total_reactions || 0) }}</p>
              </div>
              <div class="p-5">
                <div class="mb-5 flex items-center justify-between text-[#8d97a2]"><span class="text-sm">Reactions / Second</span><Zap :size="18" aria-hidden="true" /></div>
                <p class="font-mono text-4xl font-semibold tabular-nums text-[#f0eadc]">{{ liveRate || selected?.runtime?.reactions_per_second || 0 }}</p>
              </div>
            </div>
          </section>
          <div class="mt-4 grid gap-4 xl:grid-cols-[1.2fr_.8fr]">
            <AdminDisplayStatus :displays="displays" />
            <section class="panel-card">
              <header class="mb-4 border-b border-[#2a3038] pb-4">
                <h2 class="panel-heading"><ShieldCheck :size="18" aria-hidden="true" /> Room Readiness</h2>
              </header>
              <dl v-if="selected" class="divide-y divide-[#2a3038] text-sm">
                <div class="flex justify-between gap-4 py-3"><dt class="text-[#818a95]">Room State</dt><dd class="data-value">{{ roomStatusLabel }}</dd></div>
                <div class="flex justify-between gap-4 py-3"><dt class="text-[#818a95]">Audience Access</dt><dd class="data-value inline-flex items-center gap-1.5"><LockKeyhole :size="14" aria-hidden="true" /> {{ selected.locked ? 'Locked' : 'Open' }}</dd></div>
                <div class="flex justify-between gap-4 py-3"><dt class="text-[#818a95]">Unique Students</dt><dd class="data-value">{{ formatNumber(livePresence.unique_clients) }}</dd></div>
                <div class="flex justify-between gap-4 py-3"><dt class="text-[#818a95]">Connected Displays</dt><dd class="data-value">{{ livePresence.displays }}</dd></div>
              </dl>
              <p v-else class="py-6 text-sm text-[#818a95]">Create a room to begin event setup.</p>
            </section>
          </div>
        </template>

        <template v-else-if="nav==='Live Control'">
          <div v-if="selected" class="grid gap-4 xl:grid-cols-[minmax(0,1fr)_23rem]">
            <AdminLiveControls :room-code="selected.code" :status="selected.status" :locked="selected.locked" :mode="selected.display_mode" :background="selected.display_background || '#09090b'" :disabled="!controlsReady" @command="command" />
            <div class="space-y-4">
              <AdminDisplayStatus :displays="displays" />
              <AdminReactionSettings :reactions="selected.reactions" @save="saveReactions" />
              <section class="panel-card">
                <header class="mb-4 border-b border-[#2a3038] pb-4">
                  <h2 class="panel-heading"><KeyRound :size="18" aria-hidden="true" /> Display Authorization</h2>
                </header>
                <label class="label" for="display-name">Display Name</label>
                <input id="display-name" v-model="displayName" class="input" name="display-name" maxlength="120" autocomplete="off" placeholder="Main Videotron…">
                <button class="btn-primary mt-3 w-full" @click="authorizeDisplay"><Link2 :size="17" aria-hidden="true" /> Generate Secure Link</button>
                <div v-if="displayLink" class="mt-3 border border-[#5b4a27] bg-[#211d14] p-3 text-xs text-[#d9c694]">
                  <p class="break-all font-mono">{{ displayLink }}</p>
                  <button class="mt-3 inline-flex items-center gap-1.5 font-semibold text-[#f0cc80] hover:text-[#ffe0a0] focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-[#d6a84b]" @click="copyDisplayLink"><Copy :size="14" aria-hidden="true" /> Copy Link</button>
                  <p class="mt-2 text-[#a99b78]">This token appears once. Store it securely.</p>
                </div>
                <div class="mt-4 divide-y divide-[#2a3038]">
                  <div v-for="authorization in displayAuthorizations.filter(item => !item.revoked)" :key="authorization.id" class="flex items-center justify-between gap-3 py-3 text-sm">
                    <div class="min-w-0"><p class="truncate text-[#dfe2e5]">{{ authorization.name }}</p><p class="mt-1 text-xs text-[#77818c]">{{ authorization.last_seen_at ? `Last seen ${formatDateTime(authorization.last_seen_at)}` : 'Never connected' }}</p></div>
                    <button class="btn-danger !p-2" :aria-label="`Revoke ${authorization.name}`" :title="`Revoke ${authorization.name}`" @click="revokeDisplay(authorization.id, authorization.name)"><Trash2 :size="16" aria-hidden="true" /></button>
                  </div>
                </div>
              </section>
            </div>
          </div>
          <p v-else class="panel-card text-sm text-[#818a95]">Select a room before opening live controls.</p>
        </template>

        <template v-else-if="nav==='Rooms'">
          <div class="grid gap-4 xl:grid-cols-[21rem_minmax(0,1fr)]">
            <div class="space-y-2">
              <AdminRoomCard v-for="item in admin.rooms.value" :key="item.code" :room="item" :selected="item.code===selectedCode" @select="selectedCode=$event" />
              <p v-if="!admin.rooms.value.length" class="panel-card text-sm text-[#818a95]">No rooms yet. Create one to begin.</p>
            </div>
            <section v-if="selected" class="panel-card h-fit">
              <header class="mb-5 border-b border-[#2a3038] pb-4">
                <h2 class="panel-heading"><Settings2 :size="18" aria-hidden="true" /> Room Details</h2>
              </header>
              <div class="grid gap-4 sm:grid-cols-2">
                <label><span class="label">Internal Name</span><input v-model="selected.name" class="input" name="room-name" autocomplete="off"></label>
                <label><span class="label">Audience Title</span><input v-model="selected.title" class="input" name="room-title" autocomplete="off"></label>
              </div>
              <label class="label mt-4" for="join-url">Join URL</label>
              <div class="flex gap-2">
                <input id="join-url" :value="joinURL" class="input font-mono text-sm" name="join-url" readonly>
                <button class="btn-secondary shrink-0" @click="qrFullscreen=true"><QrCode :size="17" aria-hidden="true" /> Fullscreen QR</button>
              </div>
              <div class="mt-6 flex flex-wrap justify-between gap-2 border-t border-[#2a3038] pt-4">
                <button class="btn-danger" :disabled="deletingRoom" @click="deleteRoom"><Trash2 :size="17" aria-hidden="true" /> {{ deletingRoom ? 'Deleting…' : 'Delete Room' }}</button>
                <button class="btn-primary" @click="updateRoom"><Save :size="17" aria-hidden="true" /> Save Changes</button>
              </div>
            </section>
          </div>
        </template>

        <template v-else-if="nav==='Analytics'">
          <section class="panel-card !p-0">
            <div class="grid sm:grid-cols-2 xl:grid-cols-5">
              <div class="border-b border-[#2a3038] p-4 xl:border-b-0 xl:border-r"><p class="text-sm text-[#818a95]">Total Reactions</p><p class="mt-2 font-mono text-2xl font-semibold tabular-nums">{{ formatNumber(analytics.runtime?.total_reactions || 0) }}</p></div>
              <div class="border-b border-[#2a3038] p-4 xl:border-b-0 xl:border-r"><p class="text-sm text-[#818a95]">Peak Reactions / Second</p><p class="mt-2 font-mono text-2xl font-semibold tabular-nums">{{ analytics.runtime?.peak_reactions_per_second || 0 }}</p></div>
              <div class="border-b border-[#2a3038] p-4 xl:border-b-0 xl:border-r"><p class="text-sm text-[#818a95]">Peak Concurrent</p><p class="mt-2 font-mono text-2xl font-semibold tabular-nums">{{ analytics.runtime?.peak_concurrent || 0 }}</p></div>
              <div class="border-b border-[#2a3038] p-4 xl:border-b-0 xl:border-r"><p class="text-sm text-[#818a95]">Unique Students</p><p class="mt-2 font-mono text-2xl font-semibold tabular-nums">{{ analytics.runtime?.unique_users || 0 }}</p></div>
              <div class="p-4"><p class="text-sm text-[#818a95]">Session Duration</p><p class="mt-2 font-mono text-2xl font-semibold tabular-nums">{{ sessionDuration }}</p></div>
            </div>
          </section>
          <div class="mt-4 grid gap-4 xl:grid-cols-2">
            <section class="panel-card">
              <header class="mb-5 border-b border-[#2a3038] pb-4"><h2 class="panel-heading"><CircleDot :size="18" aria-hidden="true" /> Reaction Distribution</h2></header>
              <div v-for="(count,emoji) in analytics.runtime?.reaction_counts" :key="emoji" class="mb-3 grid grid-cols-[3rem_1fr_5rem] items-center gap-3">
                <span class="text-2xl">{{ emoji }}</span>
                <div class="h-2 overflow-hidden bg-[#252c33]"><div class="h-full bg-[#d6a84b]" :style="{ width: `${Math.max(2,count/(analytics.runtime?.total_reactions||1)*100)}%` }" /></div>
                <span class="text-right font-mono text-sm tabular-nums text-[#9ca5af]">{{ formatNumber(count) }}</span>
              </div>
              <p v-if="!Object.keys(analytics.runtime?.reaction_counts||{}).length" class="text-sm text-[#818a95]">Reaction data will appear after the room starts.</p>
            </section>
            <section class="panel-card">
              <header class="mb-5 border-b border-[#2a3038] pb-4"><h2 class="panel-heading"><BarChart3 :size="18" aria-hidden="true" /> Reactions Over Time</h2></header>
              <div v-if="timeline.length" class="flex h-64 items-end gap-1 border-b border-[#343c46]" aria-label="Reaction activity chart">
                <div v-for="point in timeline" :key="point.at" class="min-w-1 flex-1 bg-[#d6a84b]/80" :style="{ height: `${Math.max(2, point.count / timelineMax * 100)}%` }" :title="`${formatChartTime(point.at)}: ${formatNumber(point.count)}`" />
              </div>
              <p v-else class="text-sm text-[#818a95]">Time-series data will appear after reactions arrive.</p>
            </section>
          </div>
        </template>

        <template v-else-if="nav==='System'">
          <AdminSystemHealth :metrics="systemMetrics" class="mb-4" />
          <AdminEventLog :events="systemEvents" />
        </template>
      </section>
    </main>

    <div v-if="showCreate" class="fixed inset-0 z-50 grid place-items-center overflow-y-auto overscroll-contain bg-black/80 p-4" role="dialog" aria-modal="true" aria-labelledby="create-room-title">
      <form class="panel-card relative w-full max-w-md" @submit.prevent="createRoom">
        <button type="button" class="absolute right-3 top-3 rounded-md p-2 text-[#8d97a2] hover:bg-[#232a32] hover:text-white focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-[#d6a84b]" aria-label="Close create room dialog" title="Close" @click="showCreate=false"><X :size="18" aria-hidden="true" /></button>
        <h2 id="create-room-title" class="panel-heading pr-10"><Plus :size="18" aria-hidden="true" /> Create Room</h2>
        <p class="mb-5 mt-1 text-sm text-[#818a95]">Set the operator name and audience-facing title.</p>
        <label class="label" for="new-room-code">Room Code <span class="font-normal text-[#6f7984]">(optional)</span></label>
        <input id="new-room-code" v-model="createForm.code" class="input mb-4 font-mono uppercase" name="room-code" maxlength="12" autocomplete="off" spellcheck="false" placeholder="GRAD26">
        <label class="label" for="new-room-name">Internal Name</label>
        <input id="new-room-name" v-model="createForm.name" class="input mb-4" name="room-name" required autocomplete="off" placeholder="Main Ceremony…">
        <label class="label" for="new-room-title">Audience Title</label>
        <input id="new-room-title" v-model="createForm.title" class="input" name="room-title" required autocomplete="off" placeholder="Graduation 2026…">
        <div class="mt-6 flex justify-end gap-2 border-t border-[#2a3038] pt-4">
          <button type="button" class="btn-secondary" @click="showCreate=false">Cancel</button>
          <button class="btn-primary" type="submit"><Plus :size="17" aria-hidden="true" /> Create Room</button>
        </div>
      </form>
    </div>

    <div v-if="qrFullscreen" class="fixed inset-0 z-50 grid place-items-center overflow-y-auto overscroll-contain bg-[#f4f1e8] p-8 text-[#101317]" role="dialog" aria-modal="true" aria-labelledby="qr-title">
      <button class="absolute right-5 top-5 rounded-md border border-black/20 p-2 hover:bg-black/5 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-[#8f6b22]" aria-label="Close QR display" title="Close" @click="qrFullscreen=false"><X :size="22" aria-hidden="true" /></button>
      <div class="text-center">
        <QrcodeVue :value="joinURL" :size="360" level="H" />
        <h2 id="qr-title" class="mt-8 text-5xl font-semibold tracking-tight text-balance">Scan to Join</h2>
        <p class="mt-3 font-mono text-3xl font-semibold text-[#8f6b22]" translate="no">{{ selected?.code }}</p>
        <p class="mt-5 text-sm text-[#5f6469]">Use the close button to return to room controls.</p>
      </div>
    </div>
  </div>
</template>
