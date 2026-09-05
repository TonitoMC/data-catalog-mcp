import { ArrowUp, Loader2 } from "lucide-react"
import * as React from "react"

import { Button } from "@/components/ui/button"
import { Textarea } from "@/components/ui/textarea"

interface ChatInputProps {
  busy: boolean
  onSubmit: (message: string) => void
}

const MIN_HEIGHT = 68
const MAX_HEIGHT = 220

function ChatInput({ busy, onSubmit }: ChatInputProps) {
  const [value, setValue] = React.useState("")
  const textareaRef = React.useRef<HTMLTextAreaElement>(null)

  const resize = React.useCallback(() => {
    const el = textareaRef.current
    if (!el) return
    el.style.height = `${MIN_HEIGHT}px`
    el.style.height = `${Math.min(Math.max(el.scrollHeight, MIN_HEIGHT), MAX_HEIGHT)}px`
  }, [])

  React.useEffect(() => {
    resize()
  }, [value, resize])

  // Autofocus on load. The field itself is never disabled — typing works
  // the whole time a turn is streaming; submit() below just blocks a
  // second concurrent request from actually going out.
  React.useEffect(() => {
    textareaRef.current?.focus()
  }, [])

  function submit() {
    const text = value.trim()
    if (!text || busy) return
    onSubmit(text)
    setValue("")
  }

  // If the user typed something while the previous turn was still
  // streaming, send it the moment that turn finishes — the placeholder
  // promises this, so it has to actually happen rather than silently
  // dropping whatever they queued up.
  const wasBusy = React.useRef(busy)
  React.useEffect(() => {
    if (wasBusy.current && !busy && value.trim()) {
      submit()
    }
    wasBusy.current = busy
  }, [busy])

  return (
    <form
      className="flex items-end gap-2 border-t border-[var(--border)] p-3"
      onSubmit={(e) => {
        e.preventDefault()
        submit()
      }}
    >
      <Textarea
        ref={textareaRef}
        rows={3}
        style={{ height: MIN_HEIGHT, maxHeight: MAX_HEIGHT }}
        placeholder={
          busy
            ? "Still answering — keep typing, it'll send once this one's done…"
            : "Ask about a dataset, a column, or a validation rule… (Enter to send, Shift+Enter for a new line)"
        }
        value={value}
        onChange={(e) => setValue(e.target.value)}
        onKeyDown={(e) => {
          if (e.key === "Enter" && !e.shiftKey) {
            e.preventDefault()
            submit()
          }
        }}
      />
      <Button
        type="submit"
        variant="primary"
        size="default"
        disabled={busy || !value.trim()}
        aria-label="Send message"
        className="h-[68px] w-12 shrink-0 disabled:cursor-not-allowed"
      >
        {busy ? <Loader2 size={16} className="animate-spin" /> : <ArrowUp size={16} />}
      </Button>
    </form>
  )
}

export { ChatInput }
