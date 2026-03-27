import { useState, useEffect } from 'react'
import { api } from '../api/client'
import { clsx } from 'clsx'
import { X, Send, Bot, Loader2 } from 'lucide-react'

export function AskAIModal() {
  const [open, setOpen] = useState(false)
  const [query, setQuery] = useState('')
  const [response, setResponse] = useState<string | null>(null)
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState<string | null>(null)

  useEffect(() => {
    const handleOpen = () => setOpen(true)
    window.addEventListener('open-ask-ai', handleOpen)
    return () => window.removeEventListener('open-ask-ai', handleOpen)
  }, [])

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

    setLoading(true)
    setError(null)
    setResponse(null)

    try {
      const result = await api.askAI(query)
      setResponse(result)
    } catch (err: any) {
      setError(err.message || 'Failed to get response from AI')
    } finally {
      setLoading(false)
    }
  }

  return (
    <div className="fixed inset-0 z-[100] flex items-center justify-center bg-void/80 backdrop-blur-sm p-4 animate-in fade-in duration-200">
      <div
        className="bg-surface border border-border w-full max-w-2xl rounded-xl shadow-2xl flex flex-col max-h-[85vh] overflow-hidden"
        onClick={(e) => e.stopPropagation()}
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
          <button
            onClick={() => setOpen(false)}
            className="p-2 text-muted hover:text-text hover:bg-elevated rounded-lg transition-colors"
          >
            <X size={18} />
          </button>
        </div>

        {/* Content Area */}
        <div className="flex-1 overflow-y-auto p-5 bg-void">
          {!response && !loading && !error && (
            <div className="flex flex-col items-center justify-center h-full text-center space-y-3 opacity-60">
              <Bot size={40} className="text-accent" />
              <p className="text-sm text-muted">Ask anything about your targets, vulnerabilities, or technology stacks.</p>
            </div>
          )}

          {loading && (
            <div className="flex items-center gap-3 text-sm text-accent">
              <Loader2 size={16} className="animate-spin" />
              <span>Thinking... (this might take a few moments as the AI searches the web)</span>
            </div>
          )}

          {error && (
            <div className="p-4 bg-failed/10 border border-failed/20 text-failed rounded-lg text-sm">
              <p className="font-semibold mb-1">Error processing query</p>
              <p className="opacity-80 font-mono text-xs">{error}</p>
            </div>
          )}

          {response && (
            <div className="prose prose-invert prose-sm max-w-none text-text">
              {/* Very simple markdown rendering for now, could use react-markdown if installed */}
              <div className="whitespace-pre-wrap font-sans leading-relaxed">{response}</div>
            </div>
          )}
        </div>

        {/* Input Area */}
        <div className="p-4 bg-deep border-t border-border">
          <form onSubmit={handleSubmit} className="relative flex items-end gap-3">
            <textarea
              value={query}
              onChange={(e) => setQuery(e.target.value)}
              placeholder="E.g., What are the latest CVEs for WordPress 6.4?"
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
