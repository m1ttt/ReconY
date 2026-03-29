import { Fragment, type ReactNode, useEffect, useMemo, useRef, useState } from 'react'
import { api, type AskAIMessage } from '../api/client'
import { clsx } from 'clsx'
import { X, Send, Bot, Loader2, Sparkles, Trash2, Workflow, ChevronDown, ChevronRight } from 'lucide-react'

type ChatMessage = {
  id: string
  role: 'user' | 'assistant'
  content: string
}

type LangGraphEvent = {
  event?: string
  name?: string
  data?: Record<string, unknown>
}

type LangGraphTimelineEvent = LangGraphEvent & {
  id: string
}

function describeLangGraphEvent(event: LangGraphEvent) {
  const name = event.name || ''
  const eventName = event.event || ''
  const data = event.data || {}

  if (eventName === 'on_chain_start' && name === 'LangGraph') {
    return {
      label: 'Thinking',
      detail: 'Planning the next steps for this question.',
    }
  }

  if (eventName === 'on_tool_start') {
    return {
      label: 'Searching Tavily',
      detail: typeof data.input === 'string' && data.input ? data.input : 'Looking for current external context.',
    }
  }

  if (eventName === 'on_tool_end') {
    return {
      label: 'Reading Results',
      detail: typeof data.output === 'string' && data.output ? data.output : 'Tool results received and being analyzed.',
    }
  }

  if (eventName === 'on_chat_model_stream') {
    return {
      label: 'Drafting Answer',
      detail: typeof data.text === 'string' && data.text ? data.text : 'Generating the response.',
    }
  }

  if (eventName === 'on_chat_model_start') {
    return {
      label: 'Calling Model',
      detail: 'Sending the current context to the model.',
    }
  }

  if (eventName === 'on_chat_model_end') {
    return {
      label: 'Model Completed',
      detail: 'The model finished this reasoning pass.',
    }
  }

  return {
    label: name || 'LangGraph',
    detail: typeof data.text === 'string' && data.text
      ? data.text
      : typeof data.output === 'string' && data.output
        ? data.output
        : typeof data.input === 'string' && data.input
          ? data.input
          : eventName || 'Processing event.',
  }
}

function shouldShowLangGraphEvent(event: LangGraphEvent) {
  const name = event.name || ''
  const eventName = event.event || ''

  if (eventName === 'on_chain_start' && name === 'LangGraph') return true
  if (eventName === 'on_tool_start') return true
  if (eventName === 'on_tool_end') return true
  if (eventName === 'on_chat_model_start') return true
  if (eventName === 'on_chat_model_end') return true

  return false
}

function renderInlineMarkdown(text: string) {
  const parts = text.split(/(`[^`]+`|\*\*[^*]+\*\*)/g).filter(Boolean)

  return parts.map((part, index) => {
    if (part.startsWith('`') && part.endsWith('`')) {
      return (
        <code key={index} className="rounded bg-void/80 px-1.5 py-0.5 font-mono text-[0.9em] text-accent">
          {part.slice(1, -1)}
        </code>
      )
    }
    if (part.startsWith('**') && part.endsWith('**')) {
      return <strong key={index} className="font-semibold text-heading">{part.slice(2, -2)}</strong>
    }
    return <Fragment key={index}>{part}</Fragment>
  })
}

function renderMarkdown(content: string) {
  const lines = content.replace(/\r\n/g, '\n').split('\n')
  const blocks: ReactNode[] = []
  let i = 0

  const isTableDivider = (line: string) =>
    /^\s*\|?(?:\s*:?-{3,}:?\s*\|)+\s*:?-{3,}:?\s*\|?\s*$/.test(line)
  const isTableRow = (line: string) =>
    line.includes('|') && !line.startsWith('```')
  const parseTableCells = (line: string) =>
    line.trim().replace(/^\|/, '').replace(/\|$/, '').split('|').map((cell) => cell.trim())

  while (i < lines.length) {
    const line = lines[i]

    if (!line.trim()) {
      i += 1
      continue
    }

    if (line.startsWith('```')) {
      const codeLines: string[] = []
      i += 1
      while (i < lines.length && !lines[i].startsWith('```')) {
        codeLines.push(lines[i])
        i += 1
      }
      if (i < lines.length) i += 1
      blocks.push(
        <pre key={blocks.length} className="overflow-x-auto rounded-xl border border-border bg-void px-4 py-3 text-xs text-text">
          <code>{codeLines.join('\n')}</code>
        </pre>
      )
      continue
    }

    if (line.startsWith('# ')) {
      blocks.push(<h1 key={blocks.length} className="text-lg font-semibold text-heading">{renderInlineMarkdown(line.slice(2))}</h1>)
      i += 1
      continue
    }

    if (line.startsWith('## ')) {
      blocks.push(<h2 key={blocks.length} className="text-base font-semibold text-heading">{renderInlineMarkdown(line.slice(3))}</h2>)
      i += 1
      continue
    }

    if (line.startsWith('### ')) {
      blocks.push(<h3 key={blocks.length} className="text-sm font-semibold uppercase tracking-wide text-heading/90">{renderInlineMarkdown(line.slice(4))}</h3>)
      i += 1
      continue
    }

    if (/^[-*] /.test(line)) {
      const items: string[] = []
      while (i < lines.length && /^[-*] /.test(lines[i])) {
        items.push(lines[i].slice(2))
        i += 1
      }
      blocks.push(
        <ul key={blocks.length} className="space-y-2 pl-5 text-sm text-text">
          {items.map((item, index) => (
            <li key={index} className="list-disc">{renderInlineMarkdown(item)}</li>
          ))}
        </ul>
      )
      continue
    }

    if (
      i + 1 < lines.length &&
      isTableRow(line) &&
      isTableDivider(lines[i + 1])
    ) {
      const header = parseTableCells(line)
      i += 2
      const rows: string[][] = []
      while (i < lines.length && lines[i].trim() && isTableRow(lines[i])) {
        rows.push(parseTableCells(lines[i]))
        i += 1
      }

      blocks.push(
        <div key={blocks.length} className="overflow-x-auto rounded-xl border border-border">
          <table className="min-w-full border-collapse bg-void text-left text-sm">
            <thead className="bg-surface/80">
              <tr>
                {header.map((cell, index) => (
                  <th key={index} className="border-b border-border px-3 py-2 font-semibold text-heading">
                    {renderInlineMarkdown(cell)}
                  </th>
                ))}
              </tr>
            </thead>
            <tbody>
              {rows.map((row, rowIndex) => (
                <tr key={rowIndex} className="border-b border-border last:border-b-0">
                  {header.map((_, cellIndex) => (
                    <td key={cellIndex} className="px-3 py-2 align-top text-text">
                      {renderInlineMarkdown(row[cellIndex] || '')}
                    </td>
                  ))}
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )
      continue
    }

    const paragraph: string[] = [line]
    i += 1
    while (
      i < lines.length &&
      lines[i].trim() &&
      !lines[i].startsWith('#') &&
      !lines[i].startsWith('```') &&
      !/^[-*] /.test(lines[i])
    ) {
      paragraph.push(lines[i])
      i += 1
    }

    blocks.push(
      <p key={blocks.length} className="whitespace-pre-wrap text-sm leading-7 text-text">
        {renderInlineMarkdown(paragraph.join('\n'))}
      </p>
    )
  }

  return <div className="space-y-4">{blocks}</div>
}

function StreamingCursor() {
  return <span className="ml-1 inline-block h-4 w-2 animate-pulse rounded-sm bg-accent/80 align-middle" />
}

export function AskAIModal() {
  const [open, setOpen] = useState(false)
  const [query, setQuery] = useState('')
  const [messages, setMessages] = useState<ChatMessage[]>([])
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const [status, setStatus] = useState<string | null>(null)
  const [timeline, setTimeline] = useState<LangGraphTimelineEvent[]>([])
  const [activityOpen, setActivityOpen] = useState(false)
  const scrollRef = useRef<HTMLDivElement | null>(null)
  const draftAssistantId = useRef<string | null>(null)

  const hasMessages = useMemo(() => messages.length > 0, [messages])

  useEffect(() => {
    const handleOpen = () => setOpen(true)
    window.addEventListener('open-ask-ai', handleOpen)
    return () => window.removeEventListener('open-ask-ai', handleOpen)
  }, [])

  useEffect(() => {
    if (!scrollRef.current) return
    scrollRef.current.scrollTop = scrollRef.current.scrollHeight
  }, [messages, loading, error, status])

  // Close on escape key
  useEffect(() => {
    const handleKeyDown = (e: KeyboardEvent) => {
      if (e.key === 'Escape') setOpen(false)
    }
    if (open) window.addEventListener('keydown', handleKeyDown)
    return () => window.removeEventListener('keydown', handleKeyDown)
  }, [open])

  if (!open) return null

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault()
    if (!query.trim() || loading) return

    const nextQuery = query.trim()
    const userMessage: ChatMessage = {
      id: `user-${Date.now()}`,
      role: 'user',
      content: nextQuery,
    }
    const assistantMessage: ChatMessage = {
      id: `assistant-${Date.now()}`,
      role: 'assistant',
      content: '',
    }
    const history: AskAIMessage[] = messages.map((message) => ({
      role: message.role,
      content: message.content,
    }))

    draftAssistantId.current = assistantMessage.id
    setLoading(true)
    setError(null)
    setStatus('thinking')
    setTimeline([])
    setQuery('')
    setMessages((prev) => [...prev, userMessage, assistantMessage])

    try {
      await api.askAIStream(nextQuery, history, {
        onStatus: (nextStatus) => setStatus(nextStatus),
        onChunk: (chunk) => {
          setMessages((prev) =>
            prev.map((message) =>
              message.id === draftAssistantId.current
                ? { ...message, content: message.content + chunk }
                : message
            )
          )
        },
        onDone: () => {
          setStatus('done')
        },
        onError: (message) => {
          setError(message)
          setMessages((prev) =>
            prev.map((entry) =>
              entry.id === draftAssistantId.current && !entry.content
                ? { ...entry, content: 'No response received.' }
                : entry
            )
          )
        },
        onLangGraphEvent: (event) => {
          if (!shouldShowLangGraphEvent(event)) {
            return
          }
          setTimeline((prev) => {
            const summary = describeLangGraphEvent(event)
            const last = prev[prev.length - 1]
            if (last) {
              const lastSummary = describeLangGraphEvent(last)
              if (
                last.event === event.event &&
                last.name === event.name &&
                lastSummary.label === summary.label &&
                lastSummary.detail === summary.detail
              ) {
                return prev
              }
            }
            const next = [
              ...prev,
              {
                ...event,
                id: `${Date.now()}-${prev.length}`,
              },
            ]
            return next.slice(-8)
          })
        },
      })
    } catch (err: any) {
      setError(err.message || 'Failed to get response from AI')
    } finally {
      setLoading(false)
      draftAssistantId.current = null
    }
  }

  return (
    <div className="fixed inset-y-0 right-0 z-[100] flex animate-in slide-in-from-right duration-200">
      <div
        className="flex h-full w-[min(520px,100vw)] flex-col border-l border-border bg-surface shadow-2xl"
      >
        {/* Header */}
        <div className="flex items-center justify-between px-5 py-4 border-b border-border bg-deep">
          <div className="flex items-center gap-3">
            <div className="p-2 bg-accent/10 text-accent rounded-lg border border-accent/20">
              <Bot size={20} />
            </div>
            <div>
              <h2 className="text-sm font-semibold text-heading">ReconX AI Assistant</h2>
              <p className="text-xs text-muted">Powered by LangGraph & OpenAI</p>
            </div>
          </div>
          <div className="flex items-center gap-2">
            <button
              onClick={() => setMessages([])}
              className="p-2 text-muted hover:text-text hover:bg-elevated rounded-lg transition-colors"
              title="Clear conversation"
            >
              <Trash2 size={16} />
            </button>
            <button
              onClick={() => setOpen(false)}
              className="p-2 text-muted hover:text-text hover:bg-elevated rounded-lg transition-colors"
            >
              <X size={18} />
            </button>
          </div>
        </div>

        <div className="border-b border-border bg-deep/80 px-4 py-3">
          <button
            onClick={() => setActivityOpen((value) => !value)}
            className="flex w-full items-center justify-between rounded-lg border border-border bg-surface/50 px-3 py-2 text-left transition-colors hover:bg-surface"
          >
            <div className="flex items-center gap-2 text-[11px] uppercase tracking-[0.18em] text-muted">
              <Workflow size={13} className="text-accent" />
              <span>LangGraph Activity</span>
              <span className="rounded bg-accent/10 px-1.5 py-0.5 text-[10px] text-accent">
                {timeline.length}
              </span>
            </div>
            {activityOpen ? <ChevronDown size={16} className="text-muted" /> : <ChevronRight size={16} className="text-muted" />}
          </button>

          {activityOpen && timeline.length > 0 && (
            <div className="mt-3 max-h-52 overflow-y-auto rounded-xl border border-border bg-surface/60 p-3">
              <div className="space-y-2">
                {timeline.map((event) => (
                  <div key={event.id} className="flex items-start gap-3 text-[11px]">
                    <div className="mt-1 h-2 w-2 rounded-full bg-accent/80" />
                    <div className="min-w-0">
                      <div className="font-medium text-accent">
                        {describeLangGraphEvent(event).label}
                      </div>
                      <div className="truncate text-muted">
                        {describeLangGraphEvent(event).detail}
                      </div>
                      <div className="mt-1 font-mono text-[10px] text-muted/70">
                        {event.name || 'langgraph'} {event.event ? `· ${event.event}` : ''}
                      </div>
                    </div>
                  </div>
                ))}
              </div>
            </div>
          )}
        </div>

        {/* Content Area */}
        <div ref={scrollRef} className="flex-1 overflow-y-auto bg-void px-5 py-6">
          {!hasMessages && !loading && !error && (
            <div className="flex flex-col items-center justify-center h-full text-center space-y-3 opacity-60">
              <div className="flex h-14 w-14 items-center justify-center rounded-2xl border border-accent/20 bg-accent/10 text-accent">
                <Sparkles size={24} />
              </div>
              <p className="max-w-md text-sm text-muted">Ask anything about targets, technologies, attack surface, public exposures, or what to verify next.</p>
            </div>
          )}

          {hasMessages && (
            <div className="space-y-4">
              {messages.map((message) => (
                <div
                  key={message.id}
                  className={clsx(
                    'flex',
                    message.role === 'user' ? 'justify-end' : 'justify-start'
                  )}
                >
                  <div
                    className={clsx(
                      'max-w-[85%] rounded-2xl border px-4 py-3 shadow-sm',
                      message.role === 'user'
                        ? 'border-accent/20 bg-accent/10 text-text'
                        : 'border-border bg-surface text-text'
                    )}
                  >
                    <div className="mb-2 flex items-center gap-2 text-[10px] uppercase tracking-[0.18em] text-muted">
                      {message.role === 'user' ? 'You' : 'ReconX AI'}
                    </div>
                    {message.role === 'assistant'
                      ? (
                        <div>
                          {renderMarkdown(message.content || (loading && message.id === draftAssistantId.current ? '...' : ''))}
                          {loading && message.id === draftAssistantId.current && <StreamingCursor />}
                        </div>
                      )
                      : <div className="whitespace-pre-wrap text-sm leading-7 text-text">{message.content}</div>}
                  </div>
                </div>
              ))}
            </div>
          )}

          {loading && (
            <div className="mt-4 flex items-center gap-3 text-sm text-accent">
              <Loader2 size={16} className="animate-spin" />
              <span>
                {status === 'searching'
                  ? 'Searching and analyzing...'
                  : 'Thinking... (this might take a few moments as the AI searches the web)'}
              </span>
            </div>
          )}

          {error && (
            <div className="mt-4 rounded-xl border border-failed/20 bg-failed/10 p-4 text-sm text-failed">
              <p className="font-semibold mb-1">Error processing query</p>
              <p className="opacity-80 font-mono text-xs">{error}</p>
            </div>
          )}
        </div>

        {/* Input Area */}
        <div className="p-4 bg-deep border-t border-border">
          <form onSubmit={handleSubmit} className="relative flex items-end gap-3">
            <textarea
              value={query}
              onChange={(e) => setQuery(e.target.value)}
              placeholder="Ask about a target, CVE trail, suspicious tech stack, exposed admin panel, or next pentest hypothesis..."
              className="flex-1 bg-void border border-border rounded-lg px-4 py-3 text-sm text-text placeholder:text-muted/50 focus:outline-none focus:border-accent/50 focus:ring-1 focus:ring-accent/20 resize-none min-h-[50px] max-h-[150px]"
              rows={2}
              onKeyDown={(e) => {
                if (e.key === 'Enter' && !e.shiftKey) {
                  e.preventDefault()
                  handleSubmit(e)
                }
              }}
            />
            <button
              type="submit"
              disabled={!query.trim() || loading}
              className={clsx(
                "p-3 rounded-lg flex items-center justify-center transition-colors h-[50px] w-[50px]",
                query.trim() && !loading
                  ? "bg-accent text-void hover:bg-accent-dim shadow-lg shadow-accent/20"
                  : "bg-elevated text-muted border border-border cursor-not-allowed"
              )}
            >
              {loading ? <Loader2 size={18} className="animate-spin" /> : <Send size={18} />}
            </button>
          </form>
          <div className="mt-2 text-[10px] text-center text-muted">
            Press <kbd className="px-1.5 py-0.5 bg-elevated rounded border border-border font-mono">Enter</kbd> to send, <kbd className="px-1.5 py-0.5 bg-elevated rounded border border-border font-mono">Shift + Enter</kbd> for new line
          </div>
        </div>
      </div>
    </div>
  )
}
