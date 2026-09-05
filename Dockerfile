# Builds cmd/server for the remote deployment: MCP over HTTP
# (TRANSPORT=http), catalog.yaml + data/*.parquet baked in. The local-dev
# path (stdio, spawned by client.json's "command" entries) doesn't use
# this image at all — it runs the same binary straight from bin/.

FROM golang:1.24-bookworm AS build
WORKDIR /src

# Cache module downloads separately from source changes.
COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 go build -o /out/data-catalog-mcp ./cmd/server

FROM gcr.io/distroless/static-debian12
WORKDIR /app

COPY --from=build /out/data-catalog-mcp ./data-catalog-mcp
COPY catalog/ ./catalog/
COPY data/*.parquet ./data/

ENV TRANSPORT=http
ENV CATALOG_PATH=/app/catalog/catalog.yaml
ENV DATA_DIR=/app/data

# Cloud Run sets PORT itself and expects the container to honor it;
# cmd/server already reads PORT (falling back to 8091 locally — see
# README's "Or over HTTP" section).
EXPOSE 8091

ENTRYPOINT ["/app/data-catalog-mcp"]
