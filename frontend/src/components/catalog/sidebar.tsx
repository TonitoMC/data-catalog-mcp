import { AlertTriangle, Server } from "lucide-react"

import { Badge } from "@/components/ui/badge"
import { Panel, PanelBody, PanelHeader, PanelTitle } from "@/components/ui/panel"
import type { ServerInfoState } from "@/hooks/use-server-info"

function Sidebar({ serverInfo }: { serverInfo: ServerInfoState }) {
  return (
    <div className="flex h-full flex-col gap-4 overflow-y-auto p-3">
      <Panel>
        <PanelHeader>
          <PanelTitle>Servers connected</PanelTitle>
          <Server size={13} className="text-[var(--ink-soft)]" />
        </PanelHeader>
        <PanelBody className="flex flex-col gap-3 p-2.5">
          {serverInfo.status === "loading" && (
            <p className="px-1 font-mono text-[12px] text-[var(--ink-soft)]">
              connecting to cmd/host…
            </p>
          )}

          {serverInfo.status === "error" && (
            <div className="flex items-start gap-2 border border-[var(--accent)] bg-[var(--accent-soft)] px-2.5 py-2">
              <AlertTriangle size={13} className="mt-0.5 shrink-0 text-[var(--accent)]" />
              <div className="flex flex-col gap-0.5">
                <span className="font-mono text-[11.5px] font-medium text-[var(--accent)]">
                  couldn't reach cmd/host
                </span>
                <span className="font-mono text-[10.5px] text-[var(--ink-soft)]">
                  {serverInfo.message} — is it running on :8090?
                </span>
              </div>
            </div>
          )}

          {serverInfo.status === "ready" &&
            serverInfo.servers.map((server) => (
              <div key={server.name} className="flex flex-col gap-1.5">
                <div className="flex items-center justify-between px-1">
                  <span className="font-mono text-[12px] font-medium text-[var(--ink)]">
                    {server.name}
                  </span>
                  <Badge variant="ok">{server.tools.length} tools</Badge>
                </div>
                <div className="flex flex-wrap gap-1.5 border border-[var(--border)] p-2">
                  {server.tools.map((tool) => (
                    <Badge key={tool.name} variant="secondary" title={tool.description}>
                      {tool.name}
                    </Badge>
                  ))}
                </div>
              </div>
            ))}
        </PanelBody>
      </Panel>
    </div>
  )
}

export { Sidebar }
