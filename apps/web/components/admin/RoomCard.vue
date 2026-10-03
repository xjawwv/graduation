<script setup lang="ts">
import { LockKeyhole, Radio, Sparkles, Users } from '@lucide/vue'
import type { RoomListItem } from '~/types/protocol'

const props = defineProps<{ room: RoomListItem; selected?: boolean }>()
defineEmits<{ select: [code: string] }>()
const statusLabel = computed(() => ({ ACTIVE: 'Live', PAUSED: 'Paused', ENDED: 'Ended', WAITING: 'Waiting' })[props.room.status])
const statusClass = computed(() => ({ ACTIVE: 'text-[#73cfa1]', PAUSED: 'text-[#e0b85e]', ENDED: 'text-[#818a95]', WAITING: 'text-[#93a8bd]' })[props.room.status])
</script>

<template>
  <button
    type="button"
    class="w-full rounded-md border p-4 text-left transition-colors focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-[#d6a84b]"
    :class="selected ? 'border-[#d6a84b] bg-[#211d14]' : 'border-[#2a3038] bg-[#12171c] hover:border-[#46515c] hover:bg-[#171d23]'"
    @click="$emit('select', room.code)"
  >
    <div class="flex items-start justify-between gap-3">
      <div class="min-w-0">
        <p class="truncate font-semibold text-[#ece8dd]">{{ room.name }}</p>
        <p class="mt-1 font-mono text-xs text-[#7f8994]" translate="no">{{ room.code }}</p>
      </div>
      <span class="inline-flex shrink-0 items-center gap-1.5 text-xs font-medium" :class="statusClass">
        <Radio :size="13" aria-hidden="true" />
        {{ statusLabel }}
      </span>
    </div>
    <div class="mt-4 grid grid-cols-2 gap-2 border-t border-[#2a3038] pt-3 text-xs text-[#9da6b0]">
      <span class="inline-flex items-center gap-1.5"><Users :size="14" aria-hidden="true" /> {{ (room.runtime?.presence.students || 0).toLocaleString() }}</span>
      <span class="inline-flex items-center gap-1.5"><Sparkles :size="14" aria-hidden="true" /> {{ (room.runtime?.total_reactions || 0).toLocaleString() }}</span>
      <span v-if="room.locked" class="col-span-2 inline-flex items-center gap-1.5 text-[#d7b96f]"><LockKeyhole :size="14" aria-hidden="true" /> New joins locked</span>
    </div>
  </button>
</template>
