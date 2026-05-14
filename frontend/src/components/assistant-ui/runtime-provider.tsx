import { AssistantRuntimeProvider } from '@assistant-ui/react'
import { useRemoteThreadListRuntime } from '@assistant-ui/core/react'
import { AssistantRuntimeImpl, LocalRuntimeCore } from '@assistant-ui/core/internal'
import { useAui, useAuiState } from '@assistant-ui/store'
import { useEffect, useMemo, useRef, useState, type PropsWithChildren } from 'react'

import { goAgentChatModelAdapter } from './model-adapter'
import { persistentThreadListAdapter } from './persistent-thread-adapter'

function useLocalThreadRuntimeCore(chatModel: typeof goAgentChatModelAdapter) {
  const [runtimeCore] = useState(() => new LocalRuntimeCore({
    adapters: { chatModel },
  }, undefined))

  const aui = useAui()
  const threadIdRef = useRef<string | undefined>(undefined)
  const remoteId = useAuiState((s) => s.threadListItem.remoteId)
  threadIdRef.current = remoteId

  useEffect(() => {
    runtimeCore.threads.getMainThreadRuntimeCore().__internal_load()
  }, [runtimeCore])

  useEffect(() => {
    runtimeCore.threads
      .getMainThreadRuntimeCore()
      .__internal_setGetThreadId(() => {
        const fromRef = threadIdRef.current
        if (fromRef) return fromRef
        try {
          const state = aui.threadListItem().getState()
          if (state.remoteId) return state.remoteId
          return state.externalId
        } catch {
          return undefined
        }
      })
  }, [runtimeCore, aui])

  useEffect(() => {
    return () => {
      runtimeCore.threads.getMainThreadRuntimeCore().detach()
    }
  }, [runtimeCore])

  useEffect(() => {
    runtimeCore.threads.getMainThreadRuntimeCore().__internal_setOptions({
      adapters: { chatModel },
    })
  }, [runtimeCore, chatModel])

  return useMemo(() => new AssistantRuntimeImpl(runtimeCore), [runtimeCore])
}

function RuntimeRoot({ children }: PropsWithChildren) {
  const runtime = useRemoteThreadListRuntime({
    runtimeHook: () => useLocalThreadRuntimeCore(goAgentChatModelAdapter),
    adapter: persistentThreadListAdapter,
    allowNesting: true,
  })

  return <AssistantRuntimeProvider runtime={runtime}>{children}</AssistantRuntimeProvider>
}

export function GoAgentRuntimeProvider({ children }: PropsWithChildren) {
  return <RuntimeRoot>{children}</RuntimeRoot>
}
