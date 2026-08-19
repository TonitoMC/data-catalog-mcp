# data-catalog-mcp

Read-only MCP server for data cataloging and quality validation — dataset/column
discovery, profiling, rule-based validation, and semantic search over a
YAML-declared catalog.

Datasets live locally as Parquet files; the data dictionary and quality rules
are declared by hand in `catalog/catalog.yaml`. The server never transmits raw
data — only catalog metadata (and user queries) go to the embeddings service
used for semantic search. By default that's a local [Ollama](https://ollama.com)
instance running `nomic-embed-text`, so search works fully offline.

## Status

The MCP transport (JSON-RPC 2.0 over stdio, hand-rolled — see
`internal/jsonrpc/`) and the `initialize` / `tools/list` / `tools/call`
handshake are working end to end. Tools implemented so far:

- `ping` — health check
- `list_datasets` — real, catalog- and Parquet-backed

Not yet implemented: `describe_dataset`, `profile_column`,
`validate_dataset`, `search_catalog`, `chart_column`,
`find_undocumented_columns`.

## Requirements

- Go 1.24+

## Setup

Config values have no hardcoded defaults beyond the catalog/data paths — create
your own `.env` (gitignored, along with any `.env.*` variant) with at least:

```bash
CATALOG_PATH=catalog/catalog.yaml
DATA_DIR=data
EMBEDDINGS_API_KEY=
EMBEDDINGS_API_URL=
EMBEDDINGS_MODEL=
```

### Embeddings (semantic search)

`search_catalog` needs an embeddings service reachable at `EMBEDDINGS_API_URL`.
To run one locally with Ollama:

```bash
ollama pull nomic-embed-text
```

Ollama typically runs as a background service already (check with
`systemctl is-active ollama` on Linux) — if not, `ollama serve` starts it. Then
in your `.env`:

```bash
EMBEDDINGS_API_URL=http://localhost:11434
EMBEDDINGS_MODEL=nomic-embed-text
```

No API key is needed locally; `EMBEDDINGS_API_KEY` is only for swapping in a
hosted provider later. If the service is unreachable, `search_catalog` falls
back to keyword matching instead of failing.

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

## Running

```bash
go run ./cmd/server
```

Config is read from environment variables / `.env`:

| Variable              | Default                 | Description                          |
|------------------------|--------------------------|---------------------------------------|
| `CATALOG_PATH`          | `catalog/catalog.yaml`     | Path to the catalog definition        |
| `DATA_DIR`              | `data`                     | Directory containing Parquet files    |
| `EMBEDDINGS_API_KEY`    | —                           | API key (unused for local Ollama)     |
| `EMBEDDINGS_API_URL`    | —                           | Base URL for the embeddings service   |
| `EMBEDDINGS_MODEL`      | —                           | Model name requested from the service |

## Build

```bash
go build -o bin/data-catalog-mcp ./cmd/server
```

The server itself is never run directly by hand for real use — an MCP host
launches it as a subprocess and talks to it over stdin/stdout. For a manual
sanity check without any host, you can still pipe raw JSON-RPC lines in:

```bash
printf '%s\n%s\n' \
  '{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2024-11-05"}}' \
  '{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"list_datasets","arguments":{}}}' \
  | ./bin/data-catalog-mcp
```

## Chatting with it locally (Ollama + mcphost)

To actually talk to the server in natural language using a local model
(no Claude Desktop, no API keys), use
[mcphost](https://github.com/mark3labs/mcphost) as the middle layer: it
handles the chat model (via Ollama) on one side and speaks MCP to your
server on the other.

1. Install mcphost (prebuilt binary is easiest — `go install` currently
   fails on this repo due to broken pseudo-versions in its dependency
   tree):
   ```bash
   curl -sLO https://github.com/mark3labs/mcphost/releases/latest/download/mcphost_Linux_x86_64.tar.gz
   tar xzf mcphost_Linux_x86_64.tar.gz
   mv mcphost ~/.local/bin/
   ```
2. Pull a chat-capable model in Ollama, e.g. `ollama pull gemma3` (any
   tool-calling-capable model works — `mcphost` needs that support to let
   the model decide when to call your tools).
3. Copy `mcphost.example.json` to `mcphost.json` (gitignored — it holds
   absolute, machine-specific paths) and fill in your actual repo path:
   ```bash
   cp mcphost.example.json mcphost.json
   # edit the paths inside to point at your checkout
   go build -o bin/data-catalog-mcp ./cmd/server
   ```
4. Chat:
   ```bash
   mcphost --config mcphost.json -m ollama:gemma3
   ```
   Or non-interactively:
   ```bash
   mcphost --config mcphost.json -m ollama:gemma3 -p "What datasets are available?" --quiet
   ```

You're talking to the model, not the server directly — it decides on its
own whether a question needs a tool call.

## Project layout

```
cmd/server/         entrypoint
internal/config/    env-based configuration
internal/jsonrpc/   hand-rolled JSON-RPC 2.0 framing over stdio
internal/mcp/        MCP handshake + tool dispatch (initialize, tools/list, tools/call)
internal/catalog/    catalog.yaml parsing (Dataset/Column/Rule types)
internal/data/       resolves catalog entries to Parquet files, reads row counts
internal/embeddings/ Ollama embeddings client (for the future search_catalog)
catalog/            catalog.yaml (data dictionary + quality rules)
data/               Parquet datasets (committed; raw CSVs regenerate via scripts/)
scripts/            Python helpers to build the Parquet files from raw CSVs
mcphost.example.json example MCP client config for local chat (see above)
```
