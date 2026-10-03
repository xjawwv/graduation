<script setup lang="ts">
import type { ConnectionState } from '~/types/protocol'
const props = defineProps<{ state: ConnectionState }>()
const label = computed(() => ({ connecting: 'Connecting…', connected: 'Connected', reconnecting: 'Reconnecting…', offline: 'Offline' })[props.state])
const color = computed(() => props.state === 'connected' ? 'bg-emerald-400' : props.state === 'offline' ? 'bg-red-400' : 'bg-amber-400 animate-pulse')
</script>
<template><div class="inline-flex items-center gap-2 text-sm font-medium text-zinc-400"><span class="h-2.5 w-2.5 rounded-full" :class="color" />{{ label }}</div></template>
