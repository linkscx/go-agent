import { useEffect, useState } from 'react'
import type {
  ExportedMessageRepository,
  ThreadAssistantMessage,
  ThreadAssistantMessagePart,
  ThreadUserMessage,
} from '@assistant-ui/react'
import { ThreadPrimitive, useAui, useAuiState } from '@assistant-ui/react'
import type { ReadonlyJSONObject, ReadonlyJSONValue } from 'assistant-stream/utils'

import { fetchThreadMessages, type ChatMessageVO, type RoundMessageVO } from '../../api'
import AssistantComposer from './composer'
import AssistantThreadMessage from './message'

export default function AssistantThread() {
  const aui = useAui()
  const mainThreadId = useAuiState((s) => s.threads.mainThreadId)
  const threadItems = useAuiState((s) => s.threads.threadItems)
  const messageCount = useAuiState((s) => s.thread.messages.length)
  const isLoading = useAuiState((s) => s.thread.isLoading)
  const isRunning = useAuiState((s) => s.thread.isRunning)

  const remoteId = (() => {
    if (!mainThreadId) return undefined
    const item = threadItems.find((i) => i.id === mainThreadId)
    if (item?.remoteId) return item.remoteId
    if (mainThreadId.startsWith('__LOCALID')) return undefined
    return mainThreadId
  })()

  const [hydratedRemoteId, setHydratedRemoteId] = useState<string | null>(null)
  const [hydrationError, setHydrationError] = useState<string | null>(null)

  const isInitializing = Boolean(
    mainThreadId && mainThreadId.startsWith('__LOCALID') && !remoteId,
  )

  const needsHydration = Boolean(
    remoteId && messageCount === 0 && hydratedRemoteId !== remoteId && !isRunning,
  )

  useEffect(() => {
    setHydrationError(null)
  }, [remoteId])

  useEffect(() => {
    if (!needsHydration) return
    let cancelled = false
    const loadHistory = async () => {
      try {
        const messages = await fetchThreadMessages(remoteId!)
        if (cancelled) return
        if (messages.length === 0) {
          setHydratedRemoteId(remoteId!)
          return
        }
        const repository = buildHistoryRepository(messages)
        aui.thread().import(repository)
        setHydratedRemoteId(remoteId!)
      } catch (e) {
        if (cancelled) return
        console.error('Failed to hydrate thread:', e)
        setHydrationError('Failed to load conversation history.')
      }
    }
    loadHistory()
    return () => { cancelled = true }
  }, [needsHydration, remoteId, aui.thread])

  return (
    <ThreadPrimitive.Root
      style={{
        flex: 1,
        display: 'flex',
        flexDirection: 'column',
        background: 'var(--bg)',
        overflow: 'hidden',
      }}
    >
      <ThreadPrimitive.Viewport
        style={{
          flex: 1,
          overflowY: 'auto',
          padding: '24px 24px 0',
        }}
      >
        {messageCount === 0 && !isLoading && !isRunning ? (
          <div
            style={{
              textAlign: 'center',
              color: hydrationError ? '#fca5a5' : 'var(--text-muted)',
              marginTop: 80,
              fontSize: 14,
            }}
          >
            {hydrationError
              ? 'Failed to load conversation history.'
              : isInitializing
                ? 'Initializing conversation...'
                : needsHydration
                  ? 'Loading conversation...'
                  : 'Start a conversation...'}
          </div>
        ) : null}

        <ThreadPrimitive.Messages>
          {() => <AssistantThreadMessage />}
        </ThreadPrimitive.Messages>
      </ThreadPrimitive.Viewport>

      {!needsHydration && !hydrationError ? <AssistantComposer /> : null}
    </ThreadPrimitive.Root>
  )
}

function buildHistoryRepository(history: ChatMessageVO[]): ExportedMessageRepository {
  const messages: ExportedMessageRepository['messages'] = []
  const messageMap = new Map<string, ChatMessageVO>()

  for (const item of history) {
    messageMap.set(item.id, item)
  }

  for (const item of history) {
    const createdAt = toDate(item.created_at)

    if (item.role === 'user') {
      const userMessage: ThreadUserMessage = {
        id: item.id,
        role: 'user',
        createdAt,
        content: [{ type: 'text', text: item.content }],
        attachments: [],
        metadata: {
          custom: {},
        },
      }

      messages.push({
        parentId: item.parent_message_id || null,
        message: userMessage,
      })
    } else if (item.role === 'assistant') {
      const rounds = parseRounds(item.rounds)
      const assistantMessage: ThreadAssistantMessage = {
        id: item.id,
        role: 'assistant',
        createdAt,
        content: buildAssistantPartsFromRounds(rounds, item.content),
        status: { type: 'complete', reason: 'stop' },
        metadata: {
          unstable_state: null,
          unstable_annotations: [],
          unstable_data: [],
          steps: [],
          custom: {
            backendMessageId: item.id,
          },
        },
      }

      messages.push({
        parentId: item.parent_message_id || null,
        message: assistantMessage,
      })
    }
  }

  return {
    headId: history.length > 0 ? history[history.length - 1]?.id ?? null : null,
    messages,
  }
}

function parseRounds(roundsJson: string): RoundMessageVO[] {
  try {
    const parsed = JSON.parse(roundsJson)
    if (Array.isArray(parsed)) return parsed
    return []
  } catch {
    return []
  }
}

function buildAssistantPartsFromRounds(rounds: RoundMessageVO[], assistantContent: string): ThreadAssistantMessagePart[] {
  const parts: ThreadAssistantMessagePart[] = []

  const toolCallIds: string[] = []
  for (const round of rounds) {
    if (round.role === 'assistant' && round.tool_calls?.length) {
      for (const tc of round.tool_calls) {
        toolCallIds.push(tc.id)
        parts.push({
          type: 'tool-call',
          toolCallId: tc.id,
          toolName: tc.name,
          args: parseToolArgs(tc.arguments),
          argsText: tc.arguments,
        })
      }
    }
  }

  const toolResultMap = new Map<string, string>()
  for (const round of rounds) {
    if (round.role === 'tool' && round.tool_id) {
      const content = round.content ?? ''
      for (let i = parts.length - 1; i >= 0; i--) {
        const part = parts[i]
        if (part?.type === 'tool-call' && part.toolCallId === round.tool_id) {
          parts[i] = { ...part, result: parseJSON(content) ?? content }
          break
        }
      }
    }
  }

  if (assistantContent) {
    parts.push({ type: 'text', text: assistantContent })
  }

  return parts
}

function parseToolArgs(argsText: string): ReadonlyJSONObject {
  const parsed = parseJSON(argsText)
  if (typeof parsed === 'object' && parsed !== null && !Array.isArray(parsed)) {
    return parsed as ReadonlyJSONObject
  }
  if (!argsText.trim()) return {}
  return { raw: (parsed ?? argsText) as ReadonlyJSONValue }
}

function parseJSON(value: string): unknown {
  if (!value.trim()) return undefined
  try {
    return JSON.parse(value)
  } catch {
    return undefined
  }
}

function toDate(value: number | string): Date {
  if (typeof value === 'number') return new Date(value * 1000)
  const date = new Date(value)
  return isNaN(date.getTime()) ? new Date() : date
}
