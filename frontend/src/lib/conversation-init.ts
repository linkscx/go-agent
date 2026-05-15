import { createThread, type ConversationVO } from '../api'

let singletonConversation: ConversationVO | null = null
let initializingPromise: Promise<ConversationVO> | null = null

export function initializeConversation(localId: string): Promise<ConversationVO> {
  console.log('[conversation-init] initializeConversation called with localId:', localId)
  console.log('[conversation-init] singletonConversation:', singletonConversation ? singletonConversation.id : 'null')
  console.log('[conversation-init] initializingPromise:', initializingPromise ? 'exists' : 'null')

  if (singletonConversation) {
    console.log('[conversation-init] returning singleton conversation:', singletonConversation.id)
    return Promise.resolve(singletonConversation)
  }

  if (initializingPromise) {
    console.log('[conversation-init] returning existing initializingPromise')
    return initializingPromise
  }

  console.log('[conversation-init] creating new conversation')
  const promise = createThread().then((conversation) => {
    console.log('[conversation-init] conversation created:', conversation.id)
    singletonConversation = conversation
    initializingPromise = null
    return conversation
  }).catch((err) => {
    console.error('[conversation-init] error creating conversation:', err)
    initializingPromise = null
    throw err
  })

  initializingPromise = promise
  return promise
}

export function getResolvedConversation(localId: string): ConversationVO | undefined {
  return singletonConversation || undefined
}

export function clearConversationCache(): void {
  singletonConversation = null
  initializingPromise = null
}
