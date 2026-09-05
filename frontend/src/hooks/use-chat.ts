import * as React from "react"

import { streamChat, type WireMessage } from "@/lib/chat"

export interface ToolCallState {
  name: string
  arguments?: Record<string, unknown>
  result?: string
  status: "running" | "done"
}

// A turn's body is one ordered sequence of segments, in exactly the order
// they arrived over the stream. Nothing is ever reclassified or replaced
// after the fact — that retroactive "actually this belongs elsewhere"
// move was the whole bug (text appearing then vanishing into a reasoning
// panel, tool calls always rendered before text regardless of when they
// actually happened). Each segment is tagged correctly the moment it
// arrives and stays exactly where it landed.
export type Segment =
  | { kind: "reasoning"; text: string }
  | { kind: "answer"; text: string }
  | { kind: "tool_call"; call: ToolCallState }

export interface Turn {
  role: "user" | "assistant"
  time: string
  text: string // user turns only; assistant turns use segments
  segments?: Segment[]
  streaming?: boolean
}

function nowLabel() {
  return new Date().toLocaleTimeString("en-US", { hour12: false })
}

function useChat() {
  const [turns, setTurns] = React.useState<Turn[]>([])
  const [busy, setBusy] = React.useState(false)
  const [error, setError] = React.useState<string | null>(null)
  const historyRef = React.useRef<WireMessage[]>([])

  // Mutates only the trailing assistant turn — the one currently
  // streaming — which is always the last entry while busy is true.
  function updateStreamingTurn(fn: (turn: Turn) => Turn) {
    setTurns((prev) => {
      if (prev.length === 0) return prev
      const next = [...prev]
      next[next.length - 1] = fn(next[next.length - 1])
      return next
    })
  }

  // Appends chunk to the last segment if it's already the same text kind
  // (reasoning/answer), otherwise starts a new segment — so consecutive
  // same-kind deltas merge into one block instead of fragmenting per
  // network chunk, while a kind change (or an intervening tool call)
  // still starts fresh in the right place.
  function appendText(turn: Turn, kind: "reasoning" | "answer", chunk: string): Turn {
    const segments = turn.segments ?? []
    const last = segments[segments.length - 1]
    if (last && last.kind === kind) {
      return {
        ...turn,
        segments: [...segments.slice(0, -1), { kind, text: last.text + chunk }],
      }
    }
    return { ...turn, segments: [...segments, { kind, text: chunk }] }
  }

  const send = React.useCallback(
    async (message: string) => {
      if (!message.trim() || busy) return

      setError(null)
      setBusy(true)
      setTurns((t) => [
        ...t,
        { role: "user", time: nowLabel(), text: message },
        { role: "assistant", time: nowLabel(), text: "", segments: [], streaming: true },
      ])

      await streamChat(historyRef.current, message, {
        // Deliberately a no-op: earlier versions used a round boundary to
        // move already-visible text into a separate reasoning bucket,
        // which is exactly the "shows up then disappears" bug. Segments
        // are tagged correctly the moment they arrive, so there's nothing
        // left to reclassify at a round boundary.
        onRoundStart: () => {},
        onDelta: (chunk) => {
          updateStreamingTurn((turn) => appendText(turn, "answer", chunk))
        },
        onReasoningDelta: (chunk) => {
          updateStreamingTurn((turn) => appendText(turn, "reasoning", chunk))
        },
        onToolCallStart: (name) => {
          updateStreamingTurn((turn) => ({
            ...turn,
            segments: [
              ...(turn.segments ?? []),
              { kind: "tool_call", call: { name, status: "running" } },
            ],
          }))
        },
        onToolCallResult: (name, args, result) => {
          updateStreamingTurn((turn) => {
            const segments = turn.segments ?? []
            const idx = segments.findLastIndex(
              (s) => s.kind === "tool_call" && s.call.status === "running" && !s.call.result,
            )
            if (idx === -1) return turn
            const updated = [...segments]
            updated[idx] = { kind: "tool_call", call: { name, arguments: args, result, status: "done" } }
            return { ...turn, segments: updated }
          })
        },
        onDone: (_answer, history) => {
          // Deliberately not touching segments here — they already hold
          // the complete, correctly-ordered content from every round.
          // Overwriting with just the final round's answer was the other
          // half of the "text vanishes" bug.
          historyRef.current = history
          updateStreamingTurn((turn) => ({ ...turn, streaming: false }))
          setBusy(false)
        },
        onError: (message) => {
          setError(message)
          updateStreamingTurn((turn) => ({ ...turn, streaming: false }))
          setBusy(false)
        },
      })
    },
    [busy],
  )

  return { turns, busy, error, send }
}

export { useChat }
