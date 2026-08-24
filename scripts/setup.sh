#!/usr/bin/env bash
# One-shot local setup: generates .env and client.json from their tracked
# templates (filling in this checkout's absolute path), then builds the
# server binary. Safe to re-run — never overwrites a file that already
# exists.
set -euo pipefail

cd "$(dirname "${BASH_SOURCE[0]}")/.."
root="$(pwd)"

copy_if_missing() {
	local src="$1" dst="$2"
	if [ -f "$dst" ]; then
		echo "skip: $dst already exists"
	else
		cp "$src" "$dst"
		echo "created: $dst"
	fi
}

copy_if_missing .env.example .env

if [ -f client.json ]; then
	echo "skip: client.json already exists"
else
	sed "s#/absolute/path/to/data-catalog-mcp#${root}#g" client.example.json > client.json
	echo "created: client.json (paths point at ${root})"
fi

echo "building bin/data-catalog-mcp ..."
go build -o bin/data-catalog-mcp ./cmd/server

cat <<'EOF'

Done. Next steps:
  - Edit .env if you're not using local Ollama defaults (LLM_API_URL/LLM_MODEL/LLM_API_KEY).
  - ollama pull nomic-embed-text   # embeddings for search_catalog
  - ollama pull qwen3:8b           # or any other tool-calling-capable chat model
  - go run ./cmd/chat              # start the chat REPL
EOF
