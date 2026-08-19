# data-catalog-mcp

Read-only MCP server for data cataloging and quality validation — dataset/column
discovery, profiling, rule-based validation, and semantic search over a
YAML-declared catalog.

Datasets live locally as Parquet files; the data dictionary and quality rules
are declared by hand in `catalog/catalog.yaml`. The server never transmits raw
data — only catalog metadata (and user queries) go to the external embeddings
service used for semantic search.

## Status

Early scaffolding. Config loading and server wiring are in place; tools
(`list_datasets`, `describe_dataset`, `profile_column`, `validate_dataset`,
`search_catalog`, `chart_column`, `find_undocumented_columns`) are not yet
implemented.

## Requirements

- Go 1.24+

## Setup

```bash
cp .env.example .env
# fill in EMBEDDINGS_API_KEY / EMBEDDINGS_API_URL when the search tool is wired up
```

Place your Parquet files in `data/` and write your data dictionary in
`catalog/catalog.yaml` (see the proposal for the expected shape).

## Running

```bash
go run ./cmd/server
```

Config is read from environment variables (see `.env.example` for the full
list and defaults):

| Variable              | Default                 | Description                          |
|------------------------|--------------------------|---------------------------------------|
| `CATALOG_PATH`          | `catalog/catalog.yaml`  | Path to the catalog definition        |
| `DATA_DIR`              | `data`                  | Directory containing Parquet files    |
| `EMBEDDINGS_API_KEY`    | —                        | API key for the embeddings service    |
| `EMBEDDINGS_API_URL`    | —                        | Base URL for the embeddings service   |

## Build

```bash
go build -o bin/data-catalog-mcp ./cmd/server
./bin/data-catalog-mcp
```

## Project layout

```
cmd/server/       entrypoint
internal/config/  env-based configuration
internal/mcp/      MCP server + tool registration (internal/mcp/tools/...)
catalog/          catalog.yaml (data dictionary + quality rules)
data/             local Parquet datasets
```
