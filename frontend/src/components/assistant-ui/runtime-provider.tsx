import { AssistantRuntimeProvider, useLocalRuntime } from '@assistant-ui/react'
import type { PropsWithChildren } from 'react'

import { goAgentChatModelAdapter } from './model-adapter'
import { persistentThreadHistoryAdapter } from './persistent-thread-history-adapter'
import { persistentThreadListAdapter } from './persistent-thread-adapter'
import type { ThreadHistoryAdapter } from '@assistant-ui/react'

function RuntimeRoot({ children }: PropsWithChildren) {
  // 使用持久化的 thread history adapter
  const runtime = useLocalRuntime(goAgentChatModelAdapter, {
    adapters: {
      threadList: persistentThreadListAdapter,
    },
  })

  return <AssistantRuntimeProvider runtime={runtime}>{children}</AssistantRuntimeProvider>
}

export function GoAgentRuntimeProvider({ children }: PropsWithChildren) {
  return <RuntimeRoot>{children}</RuntimeRoot>
}
