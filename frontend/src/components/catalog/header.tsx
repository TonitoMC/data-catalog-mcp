import { Badge } from "@/components/ui/badge"
import type { ServerInfoState } from "@/hooks/use-server-info"

function Header({ serverInfo }: { serverInfo: ServerInfoState }) {
  return (
    <header className="flex h-12 shrink-0 items-center justify-between border-b border-[var(--border)] px-4">
      <div className="flex items-center gap-2.5">
        <span className="font-mono text-[13px] font-semibold tracking-tight text-[var(--ink)]">
          data-catalog-mcp
        </span>
        <span className="font-mono text-[12px] text-[var(--ink-soft)]">/ chat</span>
      </div>
      <div className="flex items-center gap-2 overflow-hidden">
        {serverInfo.status === "ready" && (
          <>
            <Badge variant="ok" title={serverInfo.llmURL} className="max-w-[280px] truncate">
              ● {serverInfo.llmURL}
            </Badge>
            <Badge variant="secondary">model: {serverInfo.llmModel}</Badge>
          </>
        )}
        {serverInfo.status === "loading" && (
          <Badge variant="neutral">connecting…</Badge>
        )}
        {serverInfo.status === "error" && <Badge variant="accent">● cmd/api unreachable</Badge>}
      </div>
    </header>
  )
}

export { Header }
