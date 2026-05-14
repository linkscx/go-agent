import type {
  RemoteThreadListAdapter,
} from '@assistant-ui/react'
import { createAssistantStream } from 'assistant-stream'

import {
  THREAD_OPERATION_SUPPORT,
  archiveThread,
  createThread,
  deleteThread,
  fetchThreads,
  renameThread,
} from '../../api'

export const persistentThreadListAdapter: RemoteThreadListAdapter = {
  async list() {
    try {
      const conversations = await fetchThreads()
      const threads = conversations
        .filter(c => !c.archived)
        .map(c => ({
          status: 'regular' as const,
          remoteId: c.id,
          title: c.title,
        }))

      return { threads }
    } catch (e) {
      console.error('Failed to fetch threads from server:', e)
      return { threads: [] }
    }
  },

  async initialize(localId) {
    const conversation = await createThread()

    return {
      remoteId: conversation.id,
      externalId: localId,
    }
  },

  async fetch(remoteId) {
    const conversations = await fetchThreads()
    const serverConversation = conversations.find(c => c.id === remoteId)
    if (!serverConversation) {
      throw new Error(`Conversation ${remoteId} was not found`)
    }

    return {
      status: serverConversation.archived ? 'archived' as const : 'regular' as const,
      remoteId: serverConversation.id,
      title: serverConversation.title,
    }
  },

  async rename(remoteId, newTitle) {
    if (!THREAD_OPERATION_SUPPORT.rename) {
      assertThreadOperationSupported(
        'renameThread is not implemented by the backend yet',
        'rename',
        remoteId,
        false,
      )
    }
    await renameThread(remoteId, newTitle)
  },

  async archive(remoteId) {
    const result = await archiveThread(remoteId)
    assertThreadOperationSupported(result.message, 'archive', remoteId, THREAD_OPERATION_SUPPORT.archive)
  },

  async unarchive(_remoteId) {
    // No-op: the backend already handles this via archive toggle
  },

  async delete(remoteId) {
    if (!THREAD_OPERATION_SUPPORT.delete) {
      assertThreadOperationSupported(
        'deleteThread is not implemented by the backend yet',
        'delete',
        remoteId,
        false,
      )
    }
    await deleteThread(remoteId)
  },

  async generateTitle(remoteId, unstable_messages) {
    const title = generateConversationTitle(unstable_messages)
    renameThread(remoteId, title).catch(() => {
      // title persistence is best-effort; keep showing generated title even if the
      // server update fails
    })
    return createAssistantStream((controller) => {
      controller.appendText(title)
    })
  },
}

function generateConversationTitle(messages: readonly import('@assistant-ui/react').ThreadMessage[]): string {
  const firstUserMessage = messages.find((message) => message.role === 'user')
  const text = firstUserMessage
    ? firstUserMessage.content
        .filter((part) => part.type === 'text')
        .map((part) => part.text)
        .join(' ')
    : ''

  const normalized = text.replace(/\s+/g, ' ').trim()
  if (!normalized) return 'New Chat'
  if (normalized.length <= 60) return normalized
  return `${normalized.slice(0, 57).trimEnd()}...`
}

function assertThreadOperationSupported(
  message: string,
  operation: string,
  threadId: string,
  supported: boolean,
): void {
  if (supported) return
  throw new Error(`${message} (operation=${operation}, threadId=${threadId})`)
}