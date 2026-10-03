<script setup lang="ts">
import { ListRestart } from '@lucide/vue'
import type { SystemEvent } from '~/types/protocol'

defineProps<{ events: SystemEvent[] }>()
const timeFormatter = new Intl.DateTimeFormat(undefined, { hour: '2-digit', minute: '2-digit', second: '2-digit' })
</script>

<template>
  <section class="panel-card">
    <header class="mb-2 flex items-center justify-between border-b border-[#2a3038] pb-4">
      <h2 class="panel-heading"><ListRestart :size="18" aria-hidden="true" /> Operations Log</h2>
      <span class="text-xs text-[#7f8994]">Newest first</span>
    </header>
    <ol v-if="events.length" class="max-h-[32rem] divide-y divide-[#252c33] overflow-auto overscroll-contain">
      <li v-for="event in events" :key="event.id" class="grid grid-cols-[5.5rem_1fr] gap-4 py-3 text-sm">
        <time class="font-mono text-xs tabular-nums text-[#7f8994]" :datetime="event.created_at">{{ timeFormatter.format(new Date(event.created_at)) }}</time>
        <span class="min-w-0 break-words text-[#c9ced3]">{{ event.message }}</span>
      </li>
    </ol>
    <p v-else class="py-8 text-center text-sm text-[#7f8994]">Operational changes will appear here.</p>
  </section>
</template>
