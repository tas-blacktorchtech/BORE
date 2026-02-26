import { useState, useRef, useEffect, useCallback } from 'react'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { Send, Save, RotateCcw, Brain, ScanSearch } from 'lucide-react'
import { toast } from 'sonner'
import { api } from '@/lib/api'
import { cn } from '@/lib/utils'
import { Button } from '@/components/ui/button'
import { Textarea } from '@/components/ui/textarea'
import { Separator } from '@/components/ui/separator'
import type { ChatMessage } from '@/lib/types'

function TypingIndicator() {
  return (
    <div className="flex items-center gap-1 px-3 py-2">
      <span className="h-2 w-2 rounded-full bg-zinc-500 animate-bounce [animation-delay:-0.3s]" />
      <span className="h-2 w-2 rounded-full bg-zinc-500 animate-bounce [animation-delay:-0.15s]" />
      <span className="h-2 w-2 rounded-full bg-zinc-500 animate-bounce" />
    </div>
  )
}

export function CommanderPage() {
  const queryClient = useQueryClient()
  const [messages, setMessages] = useState<ChatMessage[]>([])
  const [inputText, setInputText] = useState('')
  const [isTyping, setIsTyping] = useState(false)
  const [brainText, setBrainText] = useState('')
  const [brainDirty, setBrainDirty] = useState(false)
  const chatEndRef = useRef<HTMLDivElement>(null)
  const inputRef = useRef<HTMLTextAreaElement>(null)

  // Load brain content
  const { data: brainData, isLoading: brainLoading } = useQuery({
    queryKey: ['brain'],
    queryFn: api.brain.get,
  })

  useEffect(() => {
    if (brainData && !brainDirty) {
      setBrainText(brainData.brain)
    }
  }, [brainData, brainDirty])

  // Save brain
  const saveBrainMutation = useMutation({
    mutationFn: () => api.brain.save(brainText),
    onSuccess: () => {
      toast.success('Brain saved')
      setBrainDirty(false)
      queryClient.invalidateQueries({ queryKey: ['brain'] })
    },
    onError: (e: Error) => toast.error(`Failed to save: ${e.message}`),
  })

  // Scan repo to generate brain
  const scanMutation = useMutation({
    mutationFn: () => api.brain.scan(),
    onSuccess: data => {
      setBrainText(data.brain)
      setBrainDirty(true)
      toast.success('Repo scanned — review the brain below and click Save')
    },
    onError: (e: Error) => toast.error(`Scan failed: ${e.message}`),
  })

  // Chat mutation
  const chatMutation = useMutation({
    mutationFn: ({ history, message }: { history: ChatMessage[]; message: string }) =>
      api.commander.chat(history, message),
    onSuccess: data => {
      setIsTyping(false)
      setMessages(prev => [
        ...prev,
        { role: 'commander', content: data.response },
      ])
    },
    onError: (e: Error) => {
      setIsTyping(false)
      toast.error(`Commander error: ${e.message}`)
    },
  })

  // Auto-scroll to bottom on new messages
  useEffect(() => {
    chatEndRef.current?.scrollIntoView({ behavior: 'smooth' })
  }, [messages, isTyping])

  const sendMessage = useCallback(() => {
    const text = inputText.trim()
    if (!text || chatMutation.isPending) return

    const userMessage: ChatMessage = { role: 'user', content: text }
    const newHistory = [...messages, userMessage]
    setMessages(newHistory)
    setInputText('')
    setIsTyping(true)

    chatMutation.mutate({ history: messages, message: text })
  }, [inputText, messages, chatMutation])

  const handleKeyDown = (e: React.KeyboardEvent<HTMLTextAreaElement>) => {
    if (e.key === 'Enter' && !e.shiftKey) {
      e.preventDefault()
      sendMessage()
    }
    // Ctrl+L or Cmd+L to clear chat
    if (e.key === 'l' && (e.ctrlKey || e.metaKey)) {
      e.preventDefault()
      setMessages([])
    }
  }

  const clearChat = () => {
    setMessages([])
    inputRef.current?.focus()
  }

  return (
    <div className="flex h-full" style={{ height: 'calc(100vh - 56px)' }}>
      {/* Chat panel (60%) */}
      <div className="flex flex-col" style={{ width: '60%' }}>
        {/* Chat header */}
        <div className="flex items-center justify-between px-4 py-3 border-b border-zinc-800">
          <h2 className="text-sm font-semibold text-zinc-100">Commander Chat</h2>
          <Button
            variant="ghost"
            size="sm"
            onClick={clearChat}
            title="Clear chat (Ctrl+L)"
          >
            <RotateCcw className="h-3 w-3 mr-1" />
            Clear
          </Button>
        </div>

        {/* Messages area */}
        <div className="flex-1 overflow-y-auto p-4 space-y-3">
          {messages.length === 0 && (
            <div className="flex flex-col items-center justify-center h-full text-zinc-600 text-sm gap-2">
              <Brain className="h-8 w-8 mb-2" />
              <p>Start a conversation with the Commander</p>
              <p className="text-xs">Press Enter to send, Shift+Enter for newline</p>
            </div>
          )}
          {messages.map((msg, i) => (
            <div
              key={i}
              className={cn(
                'flex',
                msg.role === 'user' ? 'justify-end' : 'justify-start'
              )}
            >
              <div
                className={cn(
                  'max-w-[80%] rounded-lg px-4 py-2.5 text-sm',
                  msg.role === 'user'
                    ? 'bg-zinc-700 text-zinc-100 rounded-br-sm'
                    : 'bg-zinc-800 text-zinc-200 rounded-bl-sm'
                )}
              >
                {msg.role === 'commander' && (
                  <div className="text-xs text-zinc-500 mb-1 font-semibold uppercase tracking-wider">
                    Commander
                  </div>
                )}
                <p className="whitespace-pre-wrap break-words">{msg.content}</p>
              </div>
            </div>
          ))}
          {isTyping && (
            <div className="flex justify-start">
              <div className="bg-zinc-800 rounded-lg rounded-bl-sm">
                <TypingIndicator />
              </div>
            </div>
          )}
          <div ref={chatEndRef} />
        </div>

        {/* Input area */}
        <div className="border-t border-zinc-800 p-3">
          <div className="flex gap-2 items-end">
            <Textarea
              ref={inputRef}
              value={inputText}
              onChange={e => setInputText(e.target.value)}
              onKeyDown={handleKeyDown}
              placeholder="Ask the Commander... (Enter to send, Shift+Enter for newline)"
              className="flex-1 min-h-[44px] max-h-[120px] resize-none text-sm"
              disabled={chatMutation.isPending}
            />
            <Button
              size="icon"
              onClick={sendMessage}
              disabled={!inputText.trim() || chatMutation.isPending}
              className="shrink-0 h-11 w-11"
            >
              <Send className="h-4 w-4" />
            </Button>
          </div>
        </div>
      </div>

      {/* Separator */}
      <Separator orientation="vertical" className="shrink-0" />

      {/* Brain panel (40%) */}
      <div className="flex flex-col" style={{ width: '40%' }}>
        {/* Brain header */}
        <div className="flex items-center justify-between px-4 py-3 border-b border-zinc-800">
          <div className="flex items-center gap-2">
            <Brain className="h-4 w-4 text-zinc-400" />
            <h2 className="text-sm font-semibold text-zinc-100">Commander Brain</h2>
            {brainDirty && (
              <span className="text-xs text-yellow-500">unsaved</span>
            )}
          </div>
          <div className="flex items-center gap-2">
            <Button
              variant="outline"
              size="sm"
              onClick={() => scanMutation.mutate()}
              disabled={scanMutation.isPending}
            >
              <ScanSearch className="h-3 w-3 mr-1" />
              {scanMutation.isPending ? 'Scanning...' : 'Scan Repo'}
            </Button>
            <Button
              size="sm"
              onClick={() => saveBrainMutation.mutate()}
              disabled={!brainDirty || saveBrainMutation.isPending}
            >
              <Save className="h-3 w-3 mr-1" />
              {saveBrainMutation.isPending ? 'Saving...' : 'Save'}
            </Button>
          </div>
        </div>

        {/* Brain description */}
        <div className="px-4 py-2 border-b border-zinc-800">
          <p className="text-xs text-zinc-500">
            The Commander's persistent context. Click "Scan Repo" to auto-generate, or edit manually.
          </p>
        </div>

        {/* Brain textarea */}
        <div className="flex-1 p-3 overflow-hidden">
          {brainLoading ? (
            <div className="flex items-center justify-center h-full text-zinc-500 text-sm">
              Loading...
            </div>
          ) : (
            <Textarea
              value={brainText}
              onChange={e => {
                setBrainText(e.target.value)
                setBrainDirty(true)
              }}
              placeholder="Commander's persistent knowledge and context..."
              className="h-full w-full resize-none text-sm font-mono bg-transparent border-zinc-800"
              style={{ height: '100%' }}
            />
          )}
        </div>

        {/* Keyboard hint */}
        <div className="px-4 py-2 border-t border-zinc-800">
          <p className="text-xs text-zinc-600">
            Changes are saved when you click Save. The brain persists across sessions.
          </p>
        </div>
      </div>
    </div>
  )
}
