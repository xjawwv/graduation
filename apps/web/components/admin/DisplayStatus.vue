<script setup lang="ts">
import { MonitorCheck, MonitorX } from '@lucide/vue'
import type { DisplayHealth } from '~/types/protocol'

const props = defineProps<{ displays: DisplayHealth[] }>()
const onlineCount = computed(() => props.displays.filter((display) => display.online).length)
const timeFormatter = new Intl.DateTimeFormat(undefined, { hour: '2-digit', minute: '2-digit', second: '2-digit' })
function heartbeatLabel(timestamp: number) { return timestamp ? timeFormatter.format(new Date(timestamp)) : 'Not reported' }
</script>

<template>
  <div class="panel-card">
    <header class="mb-4 flex items-center justify-between border-b border-[#2a3038] pb-4">
      <h2 class="panel-heading"><MonitorCheck :size="18" aria-hidden="true" /> Display Health</h2>
      <span class="font-mono text-xs text-[#8f99a4]">{{ onlineCount }}/{{ displays.length }} online</span>
    </header>
    <div v-if="!onlineCount" class="mb-3 flex items-start gap-3 border border-[#613735] bg-[#241919] p-3 text-sm text-[#efaaa2]" role="status">
      <MonitorX class="mt-0.5 shrink-0" :size="17" aria-hidden="true" />
      <span>No display is connected. Open an authorized display link before the event starts.</span>
    </div>
    <div v-if="displays.length" class="divide-y divide-[#2a3038]">
      <div v-for="display in displays" :key="display.id" class="flex items-center justify-between gap-4 py-3">
        <div class="min-w-0">
          <p class="truncate font-medium text-[#e6e2d8]">{{ display.name }}</p>
          <p class="mt-1 text-xs text-[#7f8994]">{{ display.online ? 'Heartbeat' : 'Last seen' }} {{ heartbeatLabel(display.last_heartbeat) }}</p>
        </div>
        <div class="shrink-0 text-right">
          <p class="inline-flex items-center gap-1.5 text-xs font-medium" :class="display.online ? 'text-[#73cfa1]' : 'text-[#ef9b92]'">
            <span class="status-dot" :class="display.online ? 'bg-[#57b88a]' : 'bg-[#d66a5e]'" />
            {{ display.online ? 'Online' : 'Offline' }}
          </p>
          <p v-if="display.online" class="mt-1 font-mono text-xs tabular-nums text-[#7f8994]">{{ display.latency_ms }} ms</p>
        </div>
      </div>
    </div>
  </div>
</template>
