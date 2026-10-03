<script setup lang="ts">
defineProps<{ message: string }>()
</script>
<template><Transition name="prompt"><div v-if="message" class="rounded-2xl border border-amber-300/30 bg-amber-300/10 px-5 py-4 text-center text-xl font-black tracking-wide text-amber-100">{{ message }}</div></Transition></template>
<style scoped>.prompt-enter-active,.prompt-leave-active{transition:.25s ease}.prompt-enter-from,.prompt-leave-to{opacity:0;transform:translateY(-8px) scale(.98)}</style>
