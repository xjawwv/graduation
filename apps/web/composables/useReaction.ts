import type { ClientMessage } from '~/types/protocol'

export function useReaction(send: (message: ClientMessage) => boolean) {
  function trigger(emoji: string) {
    if (import.meta.client) navigator.vibrate?.(12)
    return send({ type: 'reaction', emoji })
  }
  return { trigger }
}
