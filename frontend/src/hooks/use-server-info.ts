import * as React from "react"

import { fetchServers, type ServersResponse } from "@/lib/mcp"

export type ServerInfoState =
  | { status: "loading" }
  | { status: "error"; message: string }
  | ({ status: "ready" } & ServersResponse)

function useServerInfo() {
  const [state, setState] = React.useState<ServerInfoState>({ status: "loading" })

  React.useEffect(() => {
    let cancelled = false
    fetchServers()
      .then((res) => {
        if (!cancelled) setState({ status: "ready", ...res })
      })
      .catch((err: unknown) => {
        if (!cancelled) {
          setState({
            status: "error",
            message: err instanceof Error ? err.message : "unknown error",
          })
        }
      })
    return () => {
      cancelled = true
    }
  }, [])

  return state
}

export { useServerInfo }
