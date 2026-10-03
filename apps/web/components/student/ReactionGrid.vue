<script setup lang="ts">
defineProps<{ reactions: string[]; disabled?: boolean }>()
defineEmits<{ react: [emoji: string] }>()
</script>
<template><div class="grid grid-cols-2 gap-3"><StudentReactionButton v-for="emoji in reactions" :key="emoji" :emoji="emoji" :disabled="disabled" @press="$emit('react', $event)" /></div></template>
