import type { RoomState } from '~/types/protocol'

export function useRoomState(code: string) {
  const room = ref<RoomState>({ code, title: code, status: 'WAITING', locked: false })
  const canReact = computed(() => room.value.status === 'ACTIVE')
  return { room, canReact }
}
