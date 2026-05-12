import type {
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

export function persistentThreadHistoryAdapter(): ThreadHistoryAdapter {
  return {
    async load() {
      const state = loadState()
      const mainThreadId = state.mainThreadId

      if (!mainThreadId) {
        return {
          headId: null,
          messages: [],
        }
      }

      const thread = state.threads.get(mainThreadId)
      if (!thread) {
        // 如果本地没有，从服务器获取
        const conversations = await fetchThreads()
        const serverConversation = conversations.find(c => c.id === mainThreadId)
        if (!serverConversation) {
          return {
            headId: null,
            messages: [],
          }
        }

        const messages = await fetchThreadMessages(mainThreadId)
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
        state.threads.set(mainThreadId, threadState)
        saveState(state)

        return {
          headId: messages.length > 0 ? messages[messages.length - 1]?.message_id ?? null : null,
          messages: buildMessageRepository(messages),
        }
      }

      return {
        headId: thread.messages.length > 0 ? thread.messages[thread.messages.length - 1].id : null,
        messages: thread.messages.map(msg => ({
          parentId: null,
          message: msg,
        })),
      }
    },

    async append(item) {
      console.log('ThreadHistoryAdapter.append called:', item.message.role)
      const state = loadState()
      const mainThreadId = state.mainThreadId

      if (!mainThreadId) {
        // 创建新对话
        const conversation = await createThread()
        state.mainThreadId = conversation.id
        const threadState = {
          id: conversation.id,
          status: 'regular',
          remoteId: conversation.id,
          title: conversation.title || 'Untitled',
          messages: [],
        }
        state.threads.set(conversation.id, threadState)
      }

      const thread = state.threads.get(state.mainThreadId)
      if (thread) {
        thread.messages.push(item.message)
        saveState(state)
        console.log('Message appended to storage, total messages:', thread.messages.length)
      }
    },

    async resume(_options) {
      // 暂不支持 resume
      return null
    },
  }
}

function loadState(): PersistentThreadState {
  try {
    const stored = localStorage.getItem(STORAGE_KEY)
    if (stored) {
      const parsed = JSON.parse(stored) as PersistentThreadState
      // 将普通对象转换为 Map
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
    // 将 Map 转换为对象以便 JSON 序列化
    const stateToSave = {
      mainThreadId: state.mainThreadId,
      threads: Object.fromEntries(state.threads),
    }
    localStorage.setItem(STORAGE_KEY, JSON.stringify(stateToSave))
  } catch (e) {
    console.error('Failed to save state:', e)
  }
}

function buildMessageRepository(messages: any[]): Array<{ parentId: string | null, message: any }> {
  return messages.map(msg => ({
    parentId: null, // 简化处理，不处理父子关系
    message: msg,
  }))
}
