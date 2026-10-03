<script setup lang="ts">
defineProps<{ message: string }>()
</script>
<template><div class="flex h-full items-center justify-center p-[8vw] text-center"><h1 class="max-w-7xl text-[clamp(3rem,8vw,9rem)] font-black leading-[.95] tracking-tight text-white drop-shadow-2xl">{{ message || 'Congratulations!' }}</h1></div></template>
