// Package tools implements the business logic behind each MCP tool —
// composing internal/catalog (the hand-authored data dictionary) with
// internal/data (the Parquet files themselves) and, for search_catalog,
// internal/embeddings. It knows nothing about JSON-RPC or MCP;
// internal/mcp adapts these plain Go calls onto the wire.
//
// One file per tool: list_datasets.go, describe_dataset.go, and so on.
package tools

import "errors"

// ErrDatasetNotFound is returned when a tool is given a dataset name that
// isn't in the catalog, so callers can map it to a specific error (e.g.
// invalid params) rather than a generic internal error.
var ErrDatasetNotFound = errors.New("tools: dataset not found")

// ErrColumnNotFound is returned when a tool is given a column name that
// doesn't exist in the dataset's Parquet file.
var ErrColumnNotFound = errors.New("tools: column not found")
