<script setup lang="ts">
import { Users } from '@lucide/vue'
import type { DisplayMode, Presence, RoomState } from '~/types/protocol'

type ReactionCanvasHandle = { addBatch: (batch: Record<string, number>) => void; clear: () => void }
const route = useRoute()
const code = String(route.params.room_code).toUpperCase()
const token = ref('')
const mode = ref<DisplayMode>('REACTIONS')
const room = ref<RoomState>({ code, title: code, status: 'WAITING', locked: false, background: '#09090b' })
const presence = ref<Presence>({ online: 0, students: 0, admins: 0, displays: 0, unique_clients: 0 })
const announcement = ref('')
const displayBackground = ref('#09090b')
const countdownEndsAt = ref(0)
const canvas = ref<ReactionCanvasHandle | null>(null)
const fullscreen = ref(false)
const joinURL = computed(() => import.meta.client ? `${location.origin}/join/${code}` : '')
const numberFormatter = new Intl.NumberFormat()
const ws = useWebSocket(() => token.value ? { type: 'join', role: 'display', room: code, token: token.value } : null)
let announcementTimer: number | undefined
let announcementResumeMode: DisplayMode = 'REACTIONS'

function enterFullscreen() { document.documentElement.requestFullscreen?.(); fullscreen.value = true }
function setDisplayMode(nextMode: DisplayMode) {
  clearTimeout(announcementTimer)
  announcementTimer = undefined
  mode.value = nextMode
}
function showAnnouncement(message: string, duration: number) {
  if (announcementTimer === undefined) announcementResumeMode = mode.value === 'ANNOUNCEMENT' ? 'REACTIONS' : mode.value
  clearTimeout(announcementTimer)
  announcement.value = message
  mode.value = 'ANNOUNCEMENT'
  announcementTimer = window.setTimeout(() => {
    mode.value = announcementResumeMode
    announcementTimer = undefined
  }, Math.min(10, Math.max(5, duration)) * 1_000)
}
onMounted(() => {
  const queryToken = typeof route.query.token === 'string' ? route.query.token : ''
  const key = `graduation_display_token_${code}`
  token.value = queryToken || localStorage.getItem(key) || ''
  if (queryToken) { localStorage.setItem(key, queryToken); history.replaceState({}, '', `/display/${code}`) }
  ws.onMessage((message) => {
    if (message.room) {
      room.value = message.room
      if (message.room.background) displayBackground.value = message.room.background
    }
    if (message.presence) presence.value = message.presence
    if (message.type === 'display_mode' && message.mode) setDisplayMode(message.mode)
    else if (message.type === 'joined' && message.mode) mode.value = message.mode
    if (message.background) displayBackground.value = message.background
    if (message.countdown_ends_at !== undefined) countdownEndsAt.value = message.countdown_ends_at
    if (message.type === 'announcement') showAnnouncement(message.message || '', message.duration || 5)
    if (message.type === 'reaction_batch' && message.reactions && mode.value === 'REACTIONS') canvas.value?.addBatch(message.reactions)
    if (message.type === 'clear_display') canvas.value?.clear()
    if (message.type === 'joined') announcement.value = message.message || announcement.value
  })
  ws.connect()
})
onBeforeUnmount(() => clearTimeout(announcementTimer))
useHead({
  title: computed(() => `${room.value.title} · Display`),
  htmlAttrs: { class: 'display-surface' },
})
</script>

<template>
  <main class="relative h-dvh w-screen overflow-hidden text-white" :style="{ backgroundColor: displayBackground }">
    <DisplayReactionCanvas v-show="mode==='REACTIONS'" ref="canvas" />
    <DisplayAnnouncement v-if="mode==='ANNOUNCEMENT'" :message="announcement" />
    <DisplayCountdown v-else-if="mode==='COUNTDOWN'" :ends-at="countdownEndsAt" />
    <DisplayCelebration v-else-if="mode==='CELEBRATION'" />
    <DisplayQRDisplay v-else-if="mode==='QR'" :url="joinURL" :code="code" />
    <div v-else-if="mode==='BLANK'" class="h-full bg-black" />

    <div v-if="mode==='REACTIONS'" class="pointer-events-none absolute left-0 top-0 p-[3vw]">
      <p class="text-[1.2vw] font-bold uppercase tracking-[.3em] text-violet-300">Live Celebration</p>
      <h1 class="mt-1 text-[3vw] font-black">{{ room.title }}</h1>
    </div>
    <div v-if="mode!=='BLANK'" class="pointer-events-none absolute right-[3vw] top-[3vw] z-30 inline-flex items-center gap-[.7vw] border border-white/15 bg-black/45 px-[1.5vw] py-[.7vw] text-[1.3vw] font-semibold tabular-nums text-white backdrop-blur" aria-live="polite">
      <Users :size="24" aria-hidden="true" />
      {{ numberFormatter.format(presence.students) }} connected
    </div>

    <div v-if="!token" class="absolute inset-0 z-50 flex items-center justify-center bg-zinc-950 p-8 text-center"><div><p class="text-7xl">🔒</p><h1 class="mt-5 text-4xl font-black">Display Authorization Required</h1><p class="mt-3 text-zinc-400">Open the secure display link generated in the admin panel.</p></div></div>
    <div v-else-if="ws.state.value!=='connected'" class="absolute bottom-5 left-1/2 z-40 -translate-x-1/2 rounded-full bg-amber-400 px-5 py-2 text-sm font-bold text-zinc-950">{{ ws.state.value === 'reconnecting' ? 'Reconnecting…' : 'Connecting…' }}</div>
    <button v-if="!fullscreen && token" class="absolute bottom-4 right-4 z-30 rounded-lg bg-black/30 px-3 py-2 text-xs text-white/60 hover:text-white" @click="enterFullscreen">Enter Fullscreen</button>
  </main>
</template>
