<script setup lang="ts">
const props = defineProps<{ endsAt: number }>()
const now = ref(Date.now())
let timer: number | undefined
const remaining = computed(() => Math.max(0, Math.ceil((props.endsAt - now.value) / 1000)))
onMounted(() => { timer = window.setInterval(() => { now.value = Date.now() }, 200) })
onBeforeUnmount(() => clearInterval(timer))
</script>
<template><div class="flex h-full flex-col items-center justify-center"><p class="mb-6 text-[2vw] font-bold uppercase tracking-[.4em] text-violet-200">Get ready</p><div class="text-[22vw] font-black leading-none tabular-nums">{{ remaining }}</div></div></template>
