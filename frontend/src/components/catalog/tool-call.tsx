import { ChevronRight, Loader2, Wrench } from "lucide-react"
import * as React from "react"

import { Badge } from "@/components/ui/badge"
import { cn } from "@/lib/utils"

interface ToolCallProps {
  name: string
  args?: string
  result?: string
  status: "running" | "ok" | "warning"
}

function ToolCall({ name, args, result, status }: ToolCallProps) {
  // Collapsed by default, always — even while running. A page full of
  // auto-expanded call bodies (each a fetched dataset's worth of JSON)
  // was pushing the actual answer off screen. The status pill already
  // says running/done/flagged; the body is opt-in.
  const [open, setOpen] = React.useState(false)

  return (
    <div className="border border-[var(--border)] bg-[var(--surface-2)]">
      <button
        type="button"
        onClick={() => setOpen((v) => !v)}
        className="flex w-full items-center gap-2 px-3 py-2 text-left"
      >
        <ChevronRight
          size={13}
          className={cn(
            "shrink-0 text-[var(--ink-soft)] transition-transform",
            open && "rotate-90",
          )}
        />
        {status === "running" ? (
          <Loader2 size={13} className="shrink-0 animate-spin text-[var(--secondary)]" />
        ) : (
          <Wrench size={13} className="shrink-0 text-[var(--secondary)]" />
        )}
        <span className="font-mono text-[12.5px] font-medium text-[var(--ink)]">{name}</span>
        {args && <span className="truncate font-mono text-[12px] text-[var(--ink-soft)]">{args}</span>}
        <span className="ml-auto shrink-0">
          {status === "running" && <Badge variant="secondary">running</Badge>}
          {status === "ok" && <Badge variant="ok">done</Badge>}
          {status === "warning" && <Badge variant="accent">flagged</Badge>}
        </span>
      </button>
      {open && (
        <pre className="overflow-x-auto border-t border-[var(--border)] px-3 py-2.5 font-mono text-[12px] leading-relaxed text-[var(--ink-soft)]">
          {status === "running" ? "…" : result}
        </pre>
      )}
    </div>
  )
}

export { ToolCall }
