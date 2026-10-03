<script setup lang="ts">
import QrcodeVue from 'qrcode.vue'
defineProps<{ url: string; code: string }>()
</script>
<template><div class="flex h-full items-center justify-center gap-[6vw] bg-white p-[6vw] text-zinc-950"><QrcodeVue :value="url" :size="420" level="H" /><div><p class="text-[2vw] font-bold uppercase tracking-[.25em] text-violet-700">Join the celebration</p><h1 class="mt-3 text-[6vw] font-black leading-none">Scan to react</h1><p class="mt-8 text-[4vw] font-black text-violet-700">{{ code }}</p></div></div></template>
