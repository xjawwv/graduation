<script setup lang="ts">
defineProps<{ emoji: string; disabled?: boolean }>()
defineEmits<{ press: [emoji: string] }>()
</script>
<template><button type="button" :disabled="disabled" class="flex min-h-24 select-none items-center justify-center rounded-3xl border border-white/10 bg-white/[0.07] text-5xl shadow-lg shadow-black/10 transition active:scale-90 active:bg-white/15 disabled:grayscale" :aria-label="`Send ${emoji} reaction`" @click="$emit('press', emoji)">{{ emoji }}</button></template>
