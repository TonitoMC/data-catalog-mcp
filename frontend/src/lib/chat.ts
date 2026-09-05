const API_URL = import.meta.env.VITE_API_URL ?? "http://localhost:8090"

// Wire shape of internal/llm.Message — opaque to us. We never construct
// or read into these ourselves; we just hold onto whatever cmd/host handed
// back (in the "done" event) and send it right back on the next turn.
export type WireMessage = Record<string, unknown>

interface StreamEvent {
  type:
    | "round_start"
    | "delta"
    | "reasoning_delta"
    | "tool_call_start"
    | "tool_call_result"
    | "done"
    | "error"
  content?: string
  toolName?: string
  toolArgs?: Record<string, unknown>
  toolResult?: string
  answer?: string
  history?: WireMessage[]
  error?: string
}

export interface StreamHandlers {
  onRoundStart: () => void
  onDelta: (content: string) => void
  onReasoningDelta: (content: string) => void
  onToolCallStart: (name: string) => void
  onToolCallResult: (name: string, args: Record<string, unknown>, result: string) => void
  onDone: (answer: string, history: WireMessage[]) => void
  onError: (message: string) => void
}

// streamChat POSTs to cmd/host's /api/chat and reads its server-sent-event
// response as it arrives, dispatching each event to the matching handler.
// Not the browser EventSource API (that's GET-only) — this is a manual
// line-based SSE reader over a streamed POST response body.
export async function streamChat(
  history: WireMessage[],
  message: string,
  handlers: StreamHandlers,
): Promise<void> {
  let res: Response
  try {
    res = await fetch(`${API_URL}/api/chat`, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ history, message }),
    })
  } catch (err) {
    handlers.onError(err instanceof Error ? err.message : "network error")
    return
  }

  if (!res.ok || !res.body) {
    handlers.onError(`${res.status} ${res.statusText}`)
    return
  }

  const reader = res.body.getReader()
  const decoder = new TextDecoder()
  let buffer = ""

  for (;;) {
    const { value, done } = await reader.read()
    if (done) break
    buffer += decoder.decode(value, { stream: true })

    let sep: number
    while ((sep = buffer.indexOf("\n\n")) !== -1) {
      const raw = buffer.slice(0, sep).trim()
      buffer = buffer.slice(sep + 2)
      if (!raw.startsWith("data: ")) continue

      let evt: StreamEvent
      try {
        evt = JSON.parse(raw.slice("data: ".length))
      } catch {
        continue
      }

      switch (evt.type) {
        case "round_start":
          handlers.onRoundStart()
          break
        case "delta":
          handlers.onDelta(evt.content ?? "")
          break
        case "reasoning_delta":
          handlers.onReasoningDelta(evt.content ?? "")
          break
        case "tool_call_start":
          handlers.onToolCallStart(evt.toolName ?? "")
          break
        case "tool_call_result":
          handlers.onToolCallResult(evt.toolName ?? "", evt.toolArgs ?? {}, evt.toolResult ?? "")
          break
        case "done":
          handlers.onDone(evt.answer ?? "", evt.history ?? [])
          break
        case "error":
          handlers.onError(evt.error ?? "unknown error")
          break
      }
    }
  }
}
