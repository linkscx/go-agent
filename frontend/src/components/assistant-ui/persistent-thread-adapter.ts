import type {
  RemoteThreadListAdapter,
  ThreadMessage,
  ThreadMessageContent,
  ThreadMessageContentPart,
} from '@assistant-ui/react'
import { createAssistantStream } from 'assistant-stream'

import {
  THREAD_OPERATION_SUPPORT,
  archiveThread,
  createThread,
  deleteThread,
  fetchThreadMessages,
  fetchThreads,
  renameThread,
} from '../../api'

const STORAGE_KEY = 'aui_state'

export interface PersistentThreadState {
  mainThreadId: string | null
  threads: Map<string, {
    id: string
    status: 'regular' | 'archived'
    remoteId: string
    title: string
    messages: ThreadMessage[]
  }>
}

export const persistentThreadListAdapter: RemoteThreadListAdapter = {
  async list() {
    const state = loadState()
    const threads = Array.from(state.threads.values())
      .filter(t => t.status === 'regular')
      .map(t => ({
        status: t.status,
        remoteId: t.remoteId,
        title: t.title,
      }))

    return { threads }
  },

  async initialize(localId) {
    const state = loadState()
    const conversation = await createThread()
    const threadState = {
      id: conversation.id,
      status: 'regular',
      remoteId: conversation.id,
      title: conversation.title,
      messages: [],
    }
    state.threads.set(conversation.id, threadState)
    state.mainThreadId = conversation.id
    saveState(state)

    return {
      remoteId: conversation.id,
      externalId: localId,
    }
  },

  async fetch(remoteId) {
    const state = loadState()
    const thread = state.threads.get(remoteId)
    if (!thread) {
      // 如果本地没有，从服务器获取
      const conversations = await fetchThreads()
      const serverConversation = conversations.find(c => c.id === remoteId)
      if (!serverConversation) {
        throw new Error(`Conversation ${remoteId} was not found`)
      }

      const messages = await fetchThreadMessages(remoteId)
      const threadState = {
        id: serverConversation.id,
        status: serverConversation.archived ? 'archived' : 'regular',
        remoteId: serverConversation.id,
        title: serverConversation.title,
        messages: messages.map(msg => ({
          id: msg.message_id,
          role: msg.role,
          content: msg.content ? JSON.parse(msg.content) as ThreadMessageContent[] : [],
          metadata: {
            custom: {
              backendMessageId: msg.message_id,
            },
          },
        })),
      }
      state.threads.set(remoteId, threadState)
      saveState(state)

      return {
        status: threadState.status,
        remoteId: threadState.remoteId,
        title: threadState.title,
      }
    }

    return {
      status: thread.status,
      remoteId: thread.remoteId,
      title: thread.title,
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

    const state = loadState()
    const thread = state.threads.get(remoteId)
    if (thread) {
      thread.title = newTitle
      saveState(state)
    }
  },

  async archive(remoteId) {
    const result = await archiveThread(remoteId)
    assertThreadOperationSupported(result.message, 'archive', remoteId, THREAD_OPERATION_SUPPORT.archive)

    const state = loadState()
    const thread = state.threads.get(remoteId)
    if (thread) {
      thread.status = 'archived'
      saveState(state)
    }
  },

  async unarchive(remoteId) {
    const state = loadState()
    const thread = state.threads.get(remoteId)
    if (thread) {
      thread.status = 'regular'
      saveState(state)
    }
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

    const state = loadState()
    state.threads.delete(remoteId)
    if (state.mainThreadId === remoteId) {
      state.mainThreadId = null
    }
    saveState(state)
  },

  async generateTitle(_remoteId, unstable_messages) {
    const title = generateConversationTitle(unstable_messages)
    return createAssistantStream((controller) => {
      controller.appendText(title)
    })
  },
}

function loadState(): PersistentThreadState {
  try {
    const stored = localStorage.getItem(STORAGE_KEY)
    if (stored) {
      const parsed = JSON.parse(stored) as PersistentThreadState
      if (parsed.threads && typeof parsed.threads === 'object') {
        parsed.threads = new Map(Object.entries(parsed.threads))
      }
      return parsed
    }
  } catch (e) {
    console.error('Failed to load state:', e)
  }
  return {
    mainThreadId: null,
    threads: new Map(),
  }
}

function saveState(state: PersistentThreadState): void {
  try {
    const stateToSave = {
      mainThreadId: state.mainThreadId,
      threads: Object.fromEntries(state.threads),
    }
    localStorage.setItem(STORAGE_KEY, JSON.stringify(stateToSave))
  } catch (e) {
    console.error('Failed to save state:', e)
  }
}

function generateConversationTitle(messages: readonly ThreadMessage[]): string {
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
