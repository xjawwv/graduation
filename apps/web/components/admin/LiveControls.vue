<script setup lang="ts">
import {
  BellRing,
  CircleStop,
  Eraser,
  Lock,
  LockOpen,
  Megaphone,
  MessageSquareText,
  MonitorOff,
  Palette,
  PartyPopper,
  Pause,
  Play,
  QrCode,
  RadioTower,
  RectangleEllipsis,
  Send,
  Timer,
  Unplug,
  Waves,
} from '@lucide/vue'
import type { DisplayMode, RoomStatus } from '~/types/protocol'

const props = defineProps<{ roomCode: string; status: RoomStatus; locked: boolean; mode: DisplayMode; background: string; disabled?: boolean }>()
const emit = defineEmits<{ command: [type: string, payload?: Record<string, unknown>] }>()
const modes = [
  { value: 'REACTIONS' as DisplayMode, label: 'Reactions', icon: Waves },
  { value: 'ANNOUNCEMENT' as DisplayMode, label: 'Announcement', icon: RectangleEllipsis },
  { value: 'COUNTDOWN' as DisplayMode, label: 'Countdown', icon: Timer },
  { value: 'CELEBRATION' as DisplayMode, label: 'Celebration', icon: PartyPopper },
  { value: 'QR' as DisplayMode, label: 'Join QR', icon: QrCode },
  { value: 'BLANK' as DisplayMode, label: 'Blank', icon: MonitorOff },
]
const announcement = ref('')
const announcementDuration = ref(7)
const prompt = ref('')
const countdown = ref(10)
const backgroundColor = ref(props.background === 'transparent' ? '#09090b' : props.background)
const presets = ['CONGRATULATIONS! 🎓', 'MAKE SOME NOISE! 🔥', 'GIVE THEM AN APPLAUSE! 👏', 'CLASS OF 2026 🎉']
const statusLabel = computed(() => ({ WAITING: 'Waiting', ACTIVE: 'Live', PAUSED: 'Paused', ENDED: 'Ended' })[props.status])
watch(() => props.background, (value) => { if (value !== 'transparent') backgroundColor.value = value })
function setMode(mode: DisplayMode) { emit('command', 'set_display_mode', { mode, duration: mode === 'COUNTDOWN' ? countdown.value : 0 }) }
function setBackground(background: string) { emit('command', 'set_display_background', { background }) }
function toggleTransparent(event: Event) { setBackground((event.target as HTMLInputElement).checked ? 'transparent' : backgroundColor.value) }
</script>

<template>
  <fieldset :disabled="disabled" class="min-w-0 space-y-4 border-0 p-0">
    <legend class="sr-only">Live controls for {{ roomCode }}</legend>
    <div class="panel-card">
      <header class="mb-5 flex items-center justify-between border-b border-[#2a3038] pb-4">
        <h2 class="panel-heading"><RadioTower :size="18" aria-hidden="true" /> Room Control <span class="font-mono text-sm font-normal text-[#d6a84b]" translate="no">{{ roomCode }}</span></h2>
        <span class="inline-flex items-center gap-2 text-sm text-[#aab2bc]">
          <span class="status-dot" :class="status === 'ACTIVE' ? 'bg-[#57b88a]' : status === 'PAUSED' ? 'bg-[#d6a84b]' : 'bg-[#6f7883]'" />
          {{ statusLabel }}
        </span>
      </header>
      <p v-if="disabled" class="mb-4 border border-[#5b4a27] bg-[#211d14] p-3 text-sm text-[#d9c694]" role="status">Connecting controls to {{ roomCode }}…</p>
      <div class="flex flex-wrap gap-2">
        <button v-if="status === 'ACTIVE'" class="btn-secondary" @click="emit('command','pause')"><Pause :size="17" aria-hidden="true" /> Pause Reactions</button>
        <button v-else-if="status === 'PAUSED' || status === 'WAITING'" class="btn-primary" @click="emit('command','resume')"><Play :size="17" aria-hidden="true" /> {{ status === 'WAITING' ? 'Start Room' : 'Resume Reactions' }}</button>
        <button class="btn-secondary" @click="emit('command','clear_display')"><Eraser :size="17" aria-hidden="true" /> Clear Display</button>
        <button class="btn-secondary" @click="emit('command', locked ? 'unlock_room' : 'lock_room')">
          <LockOpen v-if="locked" :size="17" aria-hidden="true" />
          <Lock v-else :size="17" aria-hidden="true" />
          {{ locked ? 'Unlock Room' : 'Lock Room' }}
        </button>
      </div>
      <div class="mt-5 flex flex-wrap gap-2 border-t border-[#2a3038] pt-4">
        <button class="btn-danger" @click="emit('command','end')"><CircleStop :size="17" aria-hidden="true" /> End Session</button>
        <button class="btn-danger" @click="emit('command','disconnect_all')"><Unplug :size="17" aria-hidden="true" /> Disconnect All</button>
      </div>
    </div>

    <div class="panel-card">
      <header class="mb-5 border-b border-[#2a3038] pb-4">
        <h2 class="panel-heading"><Waves :size="18" aria-hidden="true" /> Display Mode</h2>
        <p class="mt-1 text-sm text-[#818a95]">Changes appear on every authorized screen immediately.</p>
      </header>
      <div class="grid grid-cols-2 gap-2 sm:grid-cols-3">
        <button
          v-for="item in modes"
          :key="item.value"
          class="flex min-h-20 flex-col items-start justify-between rounded-md border p-3 text-left text-sm font-medium transition-colors focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-[#d6a84b]"
          :class="mode === item.value ? 'border-[#d6a84b] bg-[#2b2416] text-[#f0cc80]' : 'border-[#343c46] bg-[#10151a] text-[#bbc2ca] hover:border-[#59636f] hover:bg-[#1a2027]'"
          :aria-pressed="mode === item.value"
          @click="setMode(item.value)"
        >
          <component :is="item.icon" :size="19" aria-hidden="true" />
          {{ item.label }}
        </button>
      </div>
      <label class="mt-4 block max-w-48" for="countdown-seconds">
        <span class="label">Countdown Seconds</span>
        <input id="countdown-seconds" v-model.number="countdown" class="input" name="countdown-seconds" type="number" min="1" max="3600" inputmode="numeric" autocomplete="off">
      </label>
      <div class="mt-5 border-t border-[#2a3038] pt-4">
        <h3 class="panel-heading"><Palette :size="18" aria-hidden="true" /> Display Background</h3>
        <p class="mt-1 text-sm text-[#818a95]">Applied only to this room’s authorized displays.</p>
        <div class="mt-3 flex flex-wrap items-center gap-4">
          <label class="flex items-center gap-3 text-sm text-[#c4cbd2]" for="display-background">
            <input id="display-background" v-model="backgroundColor" class="h-10 w-14 cursor-pointer border border-[#343c46] bg-transparent p-1 disabled:cursor-not-allowed disabled:opacity-40" name="display-background" type="color" :disabled="background === 'transparent'" @change="setBackground(backgroundColor)">
            Background Color
          </label>
          <label class="inline-flex cursor-pointer items-center gap-2 text-sm text-[#c4cbd2]">
            <input type="checkbox" :checked="background === 'transparent'" @change="toggleTransparent">
            Transparent
          </label>
        </div>
      </div>
    </div>

    <div class="panel-card">
      <header class="mb-4">
        <h2 class="panel-heading"><Megaphone :size="18" aria-hidden="true" /> Display Announcement</h2>
      </header>
      <div class="mb-3 flex flex-wrap gap-2">
        <button v-for="preset in presets" :key="preset" class="rounded-md border border-[#343c46] bg-[#10151a] px-3 py-2 text-left text-xs text-[#aeb6bf] transition-colors hover:border-[#59636f] hover:text-[#ece8dd] focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-[#d6a84b]" @click="announcement = preset">{{ preset }}</button>
      </div>
      <label for="announcement-message" class="label">Message</label>
      <textarea id="announcement-message" v-model="announcement" class="input min-h-24 resize-y" name="announcement-message" maxlength="300" autocomplete="off" placeholder="Message shown on the main display…" />
      <div class="mt-3 flex flex-wrap items-end gap-3">
        <label class="w-36" for="announcement-duration"><span class="label">Duration</span><select id="announcement-duration" v-model.number="announcementDuration" class="input" name="announcement-duration"><option v-for="seconds in 6" :key="seconds" :value="seconds + 4">{{ seconds + 4 }} seconds</option></select></label>
        <button class="btn-primary" @click="emit('command','announcement',{ message: announcement, duration: announcementDuration })"><Send :size="17" aria-hidden="true" /> Send to Display</button>
      </div>
    </div>

    <div class="panel-card">
      <header class="mb-4">
        <h2 class="panel-heading"><MessageSquareText :size="18" aria-hidden="true" /> Audience Prompt</h2>
      </header>
      <label for="audience-prompt" class="label">Prompt</label>
      <div class="flex gap-2">
        <input id="audience-prompt" v-model="prompt" class="input" name="audience-prompt" maxlength="160" autocomplete="off" placeholder="Make some noise! 🔥">
        <button class="btn-primary shrink-0" @click="emit('command','prompt',{ message: prompt })"><BellRing :size="17" aria-hidden="true" /> Send Prompt</button>
      </div>
    </div>
  </fieldset>
</template>
