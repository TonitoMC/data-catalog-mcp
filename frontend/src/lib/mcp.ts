const API_URL = import.meta.env.VITE_API_URL ?? "http://localhost:8090"

export interface ToolInfo {
  name: string
  description: string
}

export interface ServerInfo {
  name: string
  tools: ToolInfo[]
}

export interface ServersResponse {
  llmModel: string
  llmURL: string
  servers: ServerInfo[]
}

// fetchServers hits cmd/api's one config/discovery endpoint: the chat
// model + endpoint it was started with, and every MCP server it connected
// to at its own startup (with the tools each one reported via
// tools/list). Nothing here is called live per-request on our side — the
// Go process resolved all of this once and is just handing back what it
// already knows.
export async function fetchServers(): Promise<ServersResponse> {
  const res = await fetch(`${API_URL}/api/servers`)
  if (!res.ok) {
    throw new Error(`fetchServers: ${res.status} ${res.statusText}`)
  }
  return res.json()
}
