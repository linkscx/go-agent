import { AssistantRuntimeProvider } from '@assistant-ui/react'
import { useRemoteThreadListRuntime } from '@assistant-ui/core/react'
import { AssistantRuntimeImpl, LocalRuntimeCore } from '@assistant-ui/core/internal'
import { useAuiState } from '@assistant-ui/store'
import { useEffect, useMemo, useRef, useState, type PropsWithChildren } from 'react'

import { goAgentChatModelAdapter, bindThreadIdSource } from './model-adapter'
import { persistentThreadListAdapter } from './persistent-thread-adapter'

function useLocalThreadRuntimeCore(chatModel: typeof goAgentChatModelAdapter) {
  const [runtimeCore] = useState(() => new LocalRuntimeCore({
    adapters: { chatModel },
  }))

  const threadIdRef = useRef<string | undefined>(undefined)
  threadIdRef.current = useAuiState((s) => s.threadListItem.remoteId)

  useEffect(() => {
    const fn = () => threadIdRef.current
    runtimeCore.threads
      .getMainThreadRuntimeCore()
      .__internal_setGetThreadId(fn)
    bindThreadIdSource(fn)
    return () => { bindThreadIdSource(() => undefined) }
  }, [runtimeCore])

  useEffect(() => {
    return () => {
      runtimeCore.threads.getMainThreadRuntimeCore().detach()
    }
  }, [runtimeCore])

  useEffect(() => {
    runtimeCore.threads.getMainThreadRuntimeCore().__internal_setOptions({
      adapters: { chatModel },
    })
    runtimeCore.threads.getMainThreadRuntimeCore().__internal_load()
  })

  return useMemo(() => new AssistantRuntimeImpl(runtimeCore), [runtimeCore])
}

function RuntimeRoot({ children }: PropsWithChildren) {
  const runtime = useRemoteThreadListRuntime({
    runtimeHook: function LocalRuntimeHook() {
      return useLocalThreadRuntimeCore(goAgentChatModelAdapter)
    },
    adapter: persistentThreadListAdapter,
    allowNesting: true,
  })

  return <AssistantRuntimeProvider runtime={runtime}>{children}</AssistantRuntimeProvider>
}

export function GoAgentRuntimeProvider({ children }: PropsWithChildren) {
  return <RuntimeRoot>{children}</RuntimeRoot>
}
