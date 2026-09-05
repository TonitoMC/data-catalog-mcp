import { AlertTriangle } from "lucide-react"
import * as React from "react"

import { ChatInput } from "@/components/catalog/chat-input"
import { ChatMessage } from "@/components/catalog/chat-message"
import { Header } from "@/components/catalog/header"
import { Markdown } from "@/components/catalog/markdown"
import { Reasoning } from "@/components/catalog/reasoning"
import { Sidebar } from "@/components/catalog/sidebar"
import { Thinking } from "@/components/catalog/thinking"
import { ToolCall } from "@/components/catalog/tool-call"
import { useChat } from "@/hooks/use-chat"
import { useServerInfo } from "@/hooks/use-server-info"

function App() {
  const { turns, busy, error, send } = useChat()
  const serverInfo = useServerInfo()
  const scrollRef = React.useRef<HTMLDivElement>(null)

  React.useEffect(() => {
    scrollRef.current?.scrollTo({ top: scrollRef.current.scrollHeight, behavior: "smooth" })
  }, [turns])

  return (
    <div className="flex h-svh w-full flex-col bg-[var(--bg)]">
      <Header serverInfo={serverInfo} />
      <div className="flex min-h-0 flex-1">
        <aside className="hidden w-72 shrink-0 border-r border-[var(--border)] md:block">
          <Sidebar serverInfo={serverInfo} />
        </aside>

        <main className="flex min-w-0 flex-1 flex-col">
          <div ref={scrollRef} className="flex-1 overflow-y-auto">
            {turns.length === 0 && (
              <div className="flex h-full flex-col items-center justify-center gap-1.5 px-6 text-center">
                <p className="font-mono text-[13px] text-[var(--ink-soft)]">no messages yet</p>
                <p className="max-w-sm font-mono text-[11.5px] text-[var(--ink-soft)]">
                  ask something like "which datasets are about customer churn?"
                </p>
              </div>
            )}

            {turns.map((turn, i) => {
              const segments = turn.segments ?? []
              const isEmpty = segments.length === 0

              return (
                <ChatMessage key={i} role={turn.role} time={turn.time}>
                  {turn.streaming && isEmpty && <Thinking />}

                  {segments.map((seg, j) => {
                    if (seg.kind === "reasoning") return <Reasoning key={j} text={seg.text} />
                    if (seg.kind === "answer") return <Markdown key={j}>{seg.text}</Markdown>
                    const tc = seg.call
                    return (
                      <ToolCall
                        key={j}
                        name={tc.name}
                        args={tc.arguments ? JSON.stringify(tc.arguments) : undefined}
                        result={tc.result}
                        status={
                          tc.status === "running"
                            ? "running"
                            : tc.result?.toLowerCase().startsWith("error")
                              ? "warning"
                              : "ok"
                        }
                      />
                    )
                  })}

                  {turn.text && <Markdown>{turn.text}</Markdown>}
                </ChatMessage>
              )
            })}

            {error && (
              <div className="mx-4 my-3 flex items-start gap-2 border border-[var(--accent)] bg-[var(--accent-soft)] px-3 py-2.5">
                <AlertTriangle size={14} className="mt-0.5 shrink-0 text-[var(--accent)]" />
                <div className="flex flex-col gap-0.5">
                  <span className="font-mono text-[12px] font-medium text-[var(--accent)]">
                    turn failed
                  </span>
                  <span className="font-mono text-[11px] text-[var(--ink-soft)]">{error}</span>
                </div>
              </div>
            )}
          </div>

          <ChatInput busy={busy} onSubmit={send} />
        </main>
      </div>
    </div>
  )
}

export default App
