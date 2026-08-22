# data-catalog-mcp

Read-only MCP server for data cataloging and quality validation — dataset/column
discovery, profiling, rule-based validation, and semantic search over a
YAML-declared catalog — plus a small MCP client stack (multi-server router and
a local chat REPL) for actually using it.

Datasets live locally as Parquet files; the data dictionary and quality rules
are declared by hand in `catalog/catalog.yaml`. The server never transmits raw
data — only catalog metadata (and user queries) go to the embeddings service
used for semantic search. By default that's a local [Ollama](https://ollama.com)
instance running `nomic-embed-text`, so search works fully offline.

## Status

The MCP transport (JSON-RPC 2.0 over stdio, hand-rolled — see
`internal/jsonrpc/`), the `initialize` / `tools/list` / `tools/call` handshake,
and every catalog tool are implemented and working end to end:

- `ping` — health check
- `list_datasets` — lists registered datasets with description, origin, row count
- `describe_dataset` — one dataset's metadata, row count, and column dictionary
- `profile_column` — per-column stats: type, nulls, distinct count, numeric
  min/max/mean, value frequencies or samples
- `validate_dataset` — runs the catalog's quality rules (`not_null`, `unique`,
  `range`, `allowed_values`, `regex`, `type_check`, `row_count`) and reports
  violations
- `find_undocumented_columns` — Parquet columns missing from the catalog's data
  dictionary
- `search_catalog` — ranks datasets/columns by relevance to a natural-language
  query; semantic search via embeddings when configured, keyword matching
  otherwise

On top of the server, there's also a generic MCP client (`internal/client`),
a multi-server router that treats the data-catalog server as just one of
several MCP servers alongside third-party ones (filesystem, git), and a chat
REPL (`cmd/chat`) that lets a local Ollama model use all of them as tools. See
[Client & chat](#client--chat-using-it-with-a-local-llm) below.

## Requirements

- Go 1.24+
- [Ollama](https://ollama.com) running locally (for embeddings and/or chat) —
  or any other OpenAI-compatible endpoint for chat
- `npx` (Node) if you want the official filesystem MCP server
- `python3`/`pip3` if you want the official git MCP server (`mcp-server-git`)

## Setup

Config values have no hardcoded defaults beyond the catalog/data paths — create
your own `.env` (gitignored, along with any `.env.*` variant). See
`.env.example` for the full set:

```bash
CATALOG_PATH=catalog/catalog.yaml
DATA_DIR=data

EMBEDDINGS_API_URL=http://localhost:11434
EMBEDDINGS_MODEL=nomic-embed-text

LLM_API_URL=http://localhost:11434/v1
LLM_MODEL=gemma4:e4b
LLM_API_KEY=
```

### Embeddings (semantic search)

`search_catalog` needs an embeddings service reachable at `EMBEDDINGS_API_URL`.
To run one locally with Ollama:

```bash
ollama pull nomic-embed-text
```

Ollama typically runs as a background service already (check with
`systemctl is-active ollama` on Linux) — if not, `ollama serve` starts it.

`EMBEDDINGS_API_KEY` is only for swapping in a hosted provider later; unused
for local Ollama. If the embeddings service is unreachable, `search_catalog`
falls back to keyword matching instead of failing.

### Chat model

`cmd/chat` (see below) talks to any OpenAI-compatible `/chat/completions`
endpoint via `internal/llm` — there's no provider-specific code or config,
just a URL. Ollama exposes an OpenAI-compatible API at `/v1` out of the box,
so pointing at a local model is just:

```bash
ollama pull gemma4:e4b   # any tool-calling-capable model works
```

```bash
LLM_API_URL=http://localhost:11434/v1
LLM_MODEL=gemma4:e4b
LLM_API_KEY=
```

Swap in OpenAI itself, vLLM, llama.cpp server, or anything else that speaks
the same wire format by changing `LLM_API_URL` (and setting `LLM_API_KEY` if
it requires auth) — no code changes needed.

### Data

The converted Parquet files (`data/*.parquet`) are committed to the repo, so
cloning is enough to run the server — no download step required. The raw
source CSVs/zips are gitignored and only needed if you want to regenerate the
Parquet files yourself:

1. Download the raw CSVs into `data/`:
   - `Telco-Customer-Churn.csv` — [IBM / Kaggle Telco Customer Churn](https://www.kaggle.com/datasets/blastchar/telco-customer-churn)
   - `Superstore-Sales.csv` — [Kaggle Superstore Sales](https://www.kaggle.com/datasets/rohitsahoo/sales-forecasting)
   - `online_retail_II.csv` — [UCI Online Retail II](https://archive.ics.uci.edu/dataset/502/online+retail+ii)
   - `WA_Fn-UseC_-HR-Employee-Attrition.csv` — [IBM / Kaggle HR Employee Attrition](https://www.kaggle.com/datasets/pavansubhasht/ibm-hr-analytics-attrition-dataset)
2. Convert everything to Parquet with consistent names:
   ```bash
   python3 scripts/convert_to_parquet.py
   ```
3. Generate the demo "dirty" extract (seeded, reproducible):
   ```bash
   python3 scripts/generate_bad_extract.py
   ```

This produces the five files the catalog expects:

| File                                     | Dataset name                  |
|-------------------------------------------|-------------------------------|
| `telco_customer_churn.parquet`             | `telco_customer_churn`        |
| `superstore_sales.parquet`                 | `superstore_sales`             |
| `online_retail_ii.parquet`                 | `online_retail_ii`             |
| `hr_employee_attrition.parquet`            | `hr_employee_attrition`        |
| `monthly_extract_unvalidated.parquet`      | `monthly_extract_unvalidated`  |

The data dictionary and quality rules for all five live in `catalog/catalog.yaml`.

## Running the server

```bash
go run ./cmd/server
# or, built:
go build -o bin/data-catalog-mcp ./cmd/server
```

Config is read from environment variables / `.env`:

| Variable              | Default                 | Description                          |
|------------------------|--------------------------|---------------------------------------|
| `CATALOG_PATH`          | `catalog/catalog.yaml`     | Path to the catalog definition        |
| `DATA_DIR`              | `data`                     | Directory containing Parquet files    |
| `EMBEDDINGS_API_KEY`    | —                           | API key (unused for local Ollama)     |
| `EMBEDDINGS_API_URL`    | —                           | Base URL for the embeddings service   |
| `EMBEDDINGS_MODEL`      | —                           | Model name requested from the service |

The server is never run directly by hand for real use — an MCP host (or
`internal/client`, below) launches it as a subprocess and talks to it over
stdin/stdout. For a manual sanity check without any host, you can still pipe
raw JSON-RPC lines in:

```bash
printf '%s\n%s\n' \
  '{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2024-11-05"}}' \
  '{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"list_datasets","arguments":{}}}' \
  | ./bin/data-catalog-mcp
```

## Client & chat: using it with a local LLM

`internal/client` is a barebones MCP client. It's provider-agnostic in the
same sense the server is: it spawns any MCP server as a subprocess and speaks
standard JSON-RPC/stdio MCP to it — nothing here is data-catalog-specific.
`internal/client.Router` connects to a *set* of servers, keyed by name, and
routes tool calls to the right one — so this project's own server is just one
entry in that set, alongside official third-party MCP servers.

The set of servers is a config file (`client.json`, gitignored — machine-
specific absolute paths; copy `client.example.json` to start) using the same
`mcpServers` schema as `mcphost.json`:

```json
{
  "mcpServers": {
    "data-catalog": {
      "command": "/absolute/path/to/data-catalog-mcp/bin/data-catalog-mcp",
      "env": { "CATALOG_PATH": "...", "DATA_DIR": "..." }
    },
    "filesystem": {
      "command": "npx",
      "args": ["-y", "@modelcontextprotocol/server-filesystem", "/absolute/path/to/data-catalog-mcp"]
    },
    "git": {
      "command": "python3",
      "args": ["-m", "mcp_server_git", "-r", "/absolute/path/to/data-catalog-mcp"]
    }
  }
}
```

The `filesystem` and `git` entries are the official reference MCP servers —
install once:

```bash
# filesystem server: no install step, npx fetches it on first run
# git server:
pip3 install --user mcp-server-git
```

### `cmd/client` — CLI, one call at a time

```bash
go run ./cmd/client                                          # list every tool, grouped by server
go run ./cmd/client call <server> <tool> '{"json":"args"}'   # call one tool
go run ./cmd/client call data-catalog ping
go run ./cmd/client call git git_status '{"repo_path":"/abs/path"}'
```

Reads `client.json` from the working directory by default (`-config` to
override).

### `cmd/chat` — talk to a local model, it decides what to call

```bash
go run ./cmd/chat
```

Connects to every server in `client.json`, exposes their combined tool set to
the chat model configured via `LLM_API_URL`/`LLM_MODEL` (see
[Chat model](#chat-model) above), and runs a REPL: you ask in plain language,
the model decides which tool(s) to call, `internal/client.Router` executes
them, and the model answers using the results. `/exit` to quit.

```
> Do we have any data on employees?
  [tool] data-catalog__search_catalog -> [...]
Yes — hr_employee_attrition covers employee attrition, tenure, overtime, ...
```

Tool names are qualified as `<server>__<tool>` (e.g. `git__git_status`) so
tools from different servers never collide.

## Project layout

```
cmd/server/           MCP server entrypoint
cmd/client/            CLI for the multi-server MCP client (one call at a time)
cmd/chat/               local chat REPL wired to the MCP client + a chat model

internal/config/       env-based configuration for the server
internal/jsonrpc/       hand-rolled JSON-RPC 2.0 framing over stdio (bidirectional:
                        server side reads Requests/writes Responses, client side
                        writes Requests/reads Responses)
internal/mcp/            MCP handshake + tool dispatch (initialize, tools/list, tools/call)
internal/catalog/        catalog.yaml parsing (Dataset/Column/Rule types)
internal/data/            resolves catalog entries to Parquet files, reads row counts
internal/tools/           tool implementations (list/describe/profile/validate/search/...)
internal/embeddings/      Ollama embeddings client, used by search_catalog

internal/client/          MCP client: Client (one server subprocess) + Router
                          (many servers, routes tool calls by name)
internal/llm/             provider-agnostic chat client (OpenAI-compatible wire
                          format; works against Ollama's /v1, OpenAI, or similar)

catalog/                catalog.yaml (data dictionary + quality rules)
data/                    Parquet datasets (committed; raw CSVs regenerate via scripts/)
scripts/                 Python helpers to build the Parquet files from raw CSVs

client.example.json     tracked template for client.json (mcpServers config)
mcphost.example.json    tracked template for mcphost.json (alternative chat
                        frontend for the server alone — see below)
```

## Alternative: mcphost

Before `cmd/chat` existed, [mcphost](https://github.com/mark3labs/mcphost)
was used as an external chat frontend for the server alone (not the
multi-server router). It still works if you'd rather use an established tool
than this repo's own REPL:

```bash
curl -sLO https://github.com/mark3labs/mcphost/releases/latest/download/mcphost_Linux_x86_64.tar.gz
tar xzf mcphost_Linux_x86_64.tar.gz
mv mcphost ~/.local/bin/

cp mcphost.example.json mcphost.json   # edit paths inside for your checkout
go build -o bin/data-catalog-mcp ./cmd/server

mcphost --config mcphost.json -m ollama:gemma4:e4b
```
