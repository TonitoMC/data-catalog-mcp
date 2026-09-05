import type * as React from "react"

import { cn } from "@/lib/utils"

interface ChatMessageProps {
  role: "user" | "assistant"
  time: string
  children: React.ReactNode
}

function ChatMessage({ role, time, children }: ChatMessageProps) {
  const isUser = role === "user"
  return (
    <div className="flex flex-col gap-1.5 border-b border-[var(--border)] px-4 py-4 last:border-b-0">
      <div className="flex items-baseline gap-2">
        <span
          className={cn(
            "font-mono text-[11px] font-semibold tracking-[0.06em] uppercase",
            isUser ? "text-[var(--secondary)]" : "text-[var(--accent)]",
          )}
        >
          [{isUser ? "you" : "catalog"}]
        </span>
        <span className="font-mono text-[10.5px] text-[var(--ink-soft)]">{time}</span>
      </div>
      <div className="flex flex-col gap-2.5 pl-0 text-[14px] leading-relaxed text-[var(--ink)]">
        {children}
      </div>
    </div>
  )
}

export { ChatMessage }
