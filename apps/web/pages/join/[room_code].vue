<script setup lang="ts">
import type { Presence, RoomState } from '~/types/protocol'

const route = useRoute()
const code = String(route.params.room_code).toUpperCase()
const api = useApi()
const clientId = ref('')
const room = ref<RoomState>({ code, title: code, status: 'WAITING', locked: false })
const reactions = ref<string[]>([])
const presence = ref<Presence>({ online: 0, students: 0, admins: 0, displays: 0, unique_clients: 0 })
const prompt = ref('')
const floats = ref<Array<{ id: number; emoji: string; x: number }>>([])
let floatID = 0
let promptTimer: number | undefined
const ws = useWebSocket(() => clientId.value ? { type: 'join', role: 'student', room: code, client_id: clientId.value } : null)
const enabled = computed(() => room.value.status === 'ACTIVE' && ws.state.value === 'connected')
const statusMessage = computed(() => {
  if (room.value.status === 'WAITING') return 'Reactions open when the ceremony starts.'
  if (room.value.status === 'PAUSED') return 'Reactions are temporarily paused.'
  if (room.value.status === 'ENDED') return 'This event has ended. Thank you!'
  return 'Tap to cheer them on!'
})

function react(emoji: string) {
  const id = ++floatID
  floats.value.push({ id, emoji, x: 15 + Math.random() * 70 })
  if (floats.value.length > 24) floats.value.shift()
  window.setTimeout(() => { floats.value = floats.value.filter((item) => item.id !== id) }, 900)
  navigator.vibrate?.(12)
  ws.send({ type: 'reaction', emoji })
}

onMounted(async () => {
  clientId.value = localStorage.getItem('graduation_client_id') || crypto.randomUUID()
  localStorage.setItem('graduation_client_id', clientId.value)
  try {
    const initial = await api<{ code: string; title: string; status: RoomState['status']; locked: boolean; reactions: string[] }>(`/api/rooms/${code}/public`)
    room.value = initial
    reactions.value = initial.reactions
  } catch { room.value.title = 'Room not found' }
  ws.onMessage((message) => {
    if (message.room) room.value = message.room
    if (message.presence) presence.value = message.presence
    if (message.reaction_config) reactions.value = message.reaction_config
    if (message.type === 'prompt') {
      prompt.value = message.message || ''
      clearTimeout(promptTimer)
      if (prompt.value) promptTimer = window.setTimeout(() => { prompt.value = '' }, 8_000)
    }
    if (message.type === 'system' && message.code === 'room_deleted') room.value.status = 'ENDED'
  })
  ws.connect()
})
onBeforeUnmount(() => clearTimeout(promptTimer))
useHead({ title: computed(() => `${room.value.title} · Live Reactions`) })
</script>

<template>
  <main class="safe-bottom relative min-h-dvh overflow-hidden bg-[radial-gradient(circle_at_top,#312e81_0,#18181b_42%,#09090b_100%)] px-4 py-6">
    <div class="pointer-events-none fixed inset-0 z-20 overflow-hidden" aria-hidden="true">
      <span v-for="item in floats" :key="item.id" class="local-float absolute bottom-20 text-5xl" :style="{ left: `${item.x}%` }">{{ item.emoji }}</span>
    </div>
    <section class="mx-auto flex min-h-[calc(100dvh-3rem)] max-w-md flex-col">
      <header class="mb-6 text-center">
        <div class="mb-2 text-4xl">🎓</div>
        <p class="text-xs font-bold uppercase tracking-[.3em] text-violet-300">Live graduation</p>
        <h1 class="mt-2 text-3xl font-black tracking-tight">{{ room.title }}</h1>
        <div class="mt-3 flex items-center justify-center gap-2 text-zinc-300"><span class="h-2 w-2 rounded-full bg-emerald-400" />{{ presence.students.toLocaleString() }} people online</div>
      </header>
      <StudentCrowdPrompt class="mb-4" :message="prompt" />
      <p class="mb-4 text-center text-sm text-zinc-400">{{ statusMessage }}</p>
      <StudentReactionGrid class="mt-auto" :reactions="reactions" :disabled="!enabled" @react="react" />
      <div v-if="!reactions.length" class="my-auto rounded-2xl bg-white/5 p-8 text-center text-zinc-400">No reactions are configured.</div>
      <footer class="mt-6 flex justify-center"><StudentConnectionStatus :state="ws.state.value" /></footer>
    </section>
  </main>
</template>

<style scoped>
.local-float{animation:local-float .9s ease-out forwards;transform:translateX(-50%)}
@keyframes local-float{0%{opacity:0;transform:translate(-50%,20px) scale(.5)}20%{opacity:1}100%{opacity:0;transform:translate(-50%,-180px) scale(1.35) rotate(12deg)}}
</style>
