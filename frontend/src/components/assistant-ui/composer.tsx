import { useState, useEffect } from 'react'
import { Send, Square } from 'lucide-react'
import { useAui, useAuiState } from '@assistant-ui/react'

import { Button } from '../ui/button'

export default function AssistantComposer() {
  const aui = useAui()
  const [input, setInput] = useState('')
  const isDisabled = useAuiState((s) => s.thread.isDisabled)
  const isRunning = useAuiState((s) => s.thread.isRunning)

  // Sync with composer state
  const composerText = useAuiState((s) => {
    if (!s.composer.isEditing) return ''
    return s.composer.text
  })

  useEffect(() => {
    setInput(composerText)
  }, [composerText])

  const handleChange = (e: React.ChangeEvent<HTMLTextAreaElement>) => {
    const newValue = e.target.value
    setInput(newValue)

    // Update composer state
    const composer = aui.composer()
    if (!composer.getState().isEditing) {
      composer.begin()
    }
    composer.setText(newValue)
  }

  const handleKeyDown = (e: React.KeyboardEvent<HTMLTextAreaElement>) => {
    if (e.key === 'Enter' && !e.shiftKey) {
      e.preventDefault()
      if (isRunning) {
        aui.composer().cancel()
      } else {
        handleSend()
      }
    }
  }

  const handleSend = () => {
    if (!input.trim() || isDisabled) return

    const composer = aui.composer()
    if (!composer.getState().isEditing) {
      composer.begin()
      composer.setText(input)
    }
    composer.send()
    setInput('')
  }

  const handleCancel = () => {
    aui.composer().cancel()
  }

  return (
    <div
      style={{
        borderTop: '1px solid var(--border)',
        padding: '16px 24px 20px',
        background: 'var(--sidebar-bg)',
      }}
    >
      <div
        style={{
          margin: '0 auto',
          width: '100%',
          maxWidth: 960,
          display: 'flex',
          gap: 8,
          alignItems: 'flex-end',
        }}
      >
        <textarea
          value={input}
          onChange={handleChange}
          onKeyDown={handleKeyDown}
          placeholder="Send a message..."
          disabled={isDisabled}
          className="flex min-h-[56px] w-full rounded-xl border border-[var(--border)] bg-[var(--panel-bg)] px-4 py-3 text-sm text-[var(--text)] shadow-sm placeholder:text-[var(--text-muted)] focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-[var(--accent-light)] disabled:cursor-not-allowed disabled:opacity-50"
          style={{
            flex: 1,
            width: '100%',
            minWidth: 0,
            overflowY: 'auto',
            resize: 'none',
          }}
        />
        {isRunning ? (
          <Button
            size="icon"
            aria-label="Stop generating"
            onClick={handleCancel}
            style={{
              background: 'var(--accent)',
              color: '#fff',
            }}
          >
            <Square size={14} fill="#fff" />
          </Button>
        ) : (
          <Button
            size="icon"
            aria-label="Send message"
            onClick={handleSend}
            disabled={isDisabled || !input.trim()}
          >
            <Send size={16} />
          </Button>
        )}
      </div>
    </div>
  )
}
