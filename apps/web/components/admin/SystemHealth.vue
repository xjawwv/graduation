<script setup lang="ts">
import { Gauge } from '@lucide/vue'

defineProps<{ metrics: Record<string, string | number> }>()
const numberFormatter = new Intl.NumberFormat()
function metricLabel(key: string) {
  return key.replaceAll('_', ' ').replace(/\b\w/g, (character) => character.toUpperCase())
}
function metricValue(value: string | number) { return typeof value === 'number' ? numberFormatter.format(value) : value }
</script>

<template>
  <section class="panel-card">
    <header class="mb-2 flex items-center justify-between border-b border-[#2a3038] pb-4">
      <h2 class="panel-heading"><Gauge :size="18" aria-hidden="true" /> Runtime Metrics</h2>
      <span class="text-xs text-[#7f8994]">Live server process</span>
    </header>
    <div class="grid sm:grid-cols-2 lg:grid-cols-4">
      <div v-for="(value,key) in metrics" :key="key" class="border-b border-[#2a3038] py-4 sm:px-4 sm:first:pl-0 lg:border-b-0 lg:border-r lg:last:border-r-0">
        <p class="text-sm text-[#858f9a]">{{ metricLabel(String(key)) }}</p>
        <p class="mt-1 font-mono text-2xl font-semibold tabular-nums text-[#eee9dc]">{{ metricValue(value) }}</p>
      </div>
    </div>
  </section>
</template>
