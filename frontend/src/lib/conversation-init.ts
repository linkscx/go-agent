import { createThread, type ConversationVO } from '../api'

const pending = new Map<string, Promise<ConversationVO>>()

export function initializeConversation(localId: string): Promise<ConversationVO> {
  const existing = pending.get(localId)
  if (existing) return existing

  const promise = createThread().then((conversation) => {
    pending.delete(localId)
    return conversation
  }).catch((err) => {
    pending.delete(localId)
    throw err
  })

  pending.set(localId, promise)
  return promise
}