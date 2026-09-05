import * as React from "react"

import { cn } from "@/lib/utils"

function Panel({ className, ...props }: React.ComponentProps<"div">) {
  return (
    <div
      className={cn("border border-[var(--border)] bg-[var(--surface)]", className)}
      {...props}
    />
  )
}

function PanelHeader({ className, ...props }: React.ComponentProps<"div">) {
  return (
    <div
      className={cn(
        "flex items-center justify-between gap-2 border-b border-[var(--border)] px-3 py-2",
        className,
      )}
      {...props}
    />
  )
}

function PanelTitle({ className, ...props }: React.ComponentProps<"h3">) {
  return (
    <h3
      className={cn(
        "font-mono text-[11px] font-semibold tracking-[0.08em] text-[var(--ink-soft)] uppercase",
        className,
      )}
      {...props}
    />
  )
}

function PanelBody({ className, ...props }: React.ComponentProps<"div">) {
  return <div className={cn("p-3", className)} {...props} />
}

export { Panel, PanelHeader, PanelTitle, PanelBody }
