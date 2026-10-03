<script setup lang="ts">
import { ChevronDown, ChevronUp, Plus, Save, SmilePlus, Trash2 } from '@lucide/vue'

const props = defineProps<{ reactions: string[] }>()
const emit = defineEmits<{ save: [reactions: string[]] }>()
const draft = ref<string[]>([])
const custom = ref('')
watch(() => props.reactions, (value) => { draft.value = [...value] }, { immediate: true })
function move(index: number, delta: number) { const target = index + delta; if (target < 0 || target >= draft.value.length) return; [draft.value[index], draft.value[target]] = [draft.value[target], draft.value[index]] }
function add() { const value = custom.value.trim(); if (!value || draft.value.includes(value) || draft.value.length >= 16) return; draft.value.push(value); custom.value = '' }
</script>

<template>
  <div class="panel-card">
    <header class="mb-4 border-b border-[#2a3038] pb-4">
      <h2 class="panel-heading"><SmilePlus :size="18" aria-hidden="true" /> Allowed Reactions</h2>
      <p class="mt-1 text-sm text-[#818a95]">Order matches the audience controls.</p>
    </header>
    <div class="space-y-1.5">
      <div v-for="(emoji,index) in draft" :key="emoji" class="flex items-center gap-2 border border-[#2a3038] bg-[#10151a] p-2">
        <span class="w-10 text-center text-2xl" :aria-label="emoji">{{ emoji }}</span>
        <span class="text-xs text-[#77818c]">Position {{ index + 1 }}</span>
        <div class="ml-auto flex gap-1">
          <button class="btn-secondary !p-2" :disabled="index===0" :aria-label="`Move ${emoji} up`" :title="`Move ${emoji} up`" @click="move(index,-1)"><ChevronUp :size="16" aria-hidden="true" /></button>
          <button class="btn-secondary !p-2" :disabled="index===draft.length-1" :aria-label="`Move ${emoji} down`" :title="`Move ${emoji} down`" @click="move(index,1)"><ChevronDown :size="16" aria-hidden="true" /></button>
          <button class="btn-danger !p-2" :disabled="draft.length===1" :aria-label="`Remove ${emoji}`" :title="`Remove ${emoji}`" @click="draft.splice(index,1)"><Trash2 :size="16" aria-hidden="true" /></button>
        </div>
      </div>
    </div>
    <label class="label mt-4" for="custom-reaction">Add Emoji</label>
    <div class="flex gap-2">
      <input id="custom-reaction" v-model="custom" class="input" name="custom-reaction" maxlength="16" autocomplete="off" placeholder="Paste an emoji…" @keyup.enter="add">
      <button class="btn-secondary shrink-0" @click="add"><Plus :size="17" aria-hidden="true" /> Add</button>
    </div>
    <button class="btn-primary mt-4 w-full" @click="emit('save',[...draft])"><Save :size="17" aria-hidden="true" /> Save Reactions</button>
  </div>
</template>
