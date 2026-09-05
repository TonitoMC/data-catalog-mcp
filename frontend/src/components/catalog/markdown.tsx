import ReactMarkdown, { type Components } from "react-markdown"
import remarkGfm from "remark-gfm"

import { cn } from "@/lib/utils"

const components: Components = {
  p: ({ className, ...props }) => (
    <p className={cn("leading-relaxed text-[var(--ink)]", className)} {...props} />
  ),
  a: ({ className, ...props }) => (
    <a
      className={cn(
        "text-[var(--accent)] underline decoration-[var(--accent)]/40 underline-offset-2 hover:decoration-[var(--accent)]",
        className,
      )}
      target="_blank"
      rel="noreferrer"
      {...props}
    />
  ),
  strong: ({ className, ...props }) => (
    <strong className={cn("font-semibold text-[var(--ink)]", className)} {...props} />
  ),
  ul: ({ className, ...props }) => (
    <ul className={cn("list-disc space-y-1 pl-5", className)} {...props} />
  ),
  ol: ({ className, ...props }) => (
    <ol className={cn("list-decimal space-y-1 pl-5", className)} {...props} />
  ),
  li: ({ className, ...props }) => (
    <li className={cn("leading-relaxed marker:text-[var(--ink-soft)]", className)} {...props} />
  ),
  h1: ({ className, ...props }) => (
    <h1
      className={cn("font-mono text-[15px] font-semibold text-[var(--ink)]", className)}
      {...props}
    />
  ),
  h2: ({ className, ...props }) => (
    <h2
      className={cn("font-mono text-[13.5px] font-semibold text-[var(--ink)]", className)}
      {...props}
    />
  ),
  h3: ({ className, ...props }) => (
    <h3
      className={cn("font-mono text-[13px] font-semibold text-[var(--ink)]", className)}
      {...props}
    />
  ),
  blockquote: ({ className, ...props }) => (
    <blockquote
      className={cn(
        "border-l-2 border-[var(--secondary)] pl-3 text-[var(--ink-soft)] italic",
        className,
      )}
      {...props}
    />
  ),
  hr: ({ className, ...props }) => (
    <hr className={cn("border-[var(--border)]", className)} {...props} />
  ),
  table: ({ className, ...props }) => (
    <div className="overflow-x-auto border border-[var(--border)]">
      <table className={cn("w-full border-collapse text-[13px]", className)} {...props} />
    </div>
  ),
  thead: ({ className, ...props }) => (
    <thead className={cn("bg-[var(--surface-2)]", className)} {...props} />
  ),
  tbody: ({ className, ...props }) => (
    <tbody className={cn("[&>tr:last-child>td]:border-b-0", className)} {...props} />
  ),
  th: ({ className, ...props }) => (
    <th
      className={cn(
        "border-b border-[var(--border)] px-2.5 py-1.5 text-left font-mono text-[11px] tracking-[0.04em] text-[var(--ink-soft)] uppercase",
        className,
      )}
      {...props}
    />
  ),
  td: ({ className, ...props }) => (
    <td
      className={cn("border-b border-[var(--border)] px-2.5 py-1.5", className)}
      {...props}
    />
  ),
  code: ({ className, children, ...props }) => {
    const isBlock = className?.includes("language-")
    if (isBlock) {
      return (
        <code className={cn("font-mono text-[12.5px]", className)} {...props}>
          {children}
        </code>
      )
    }
    return (
      <code
        className={cn(
          "border border-[var(--border)] bg-[var(--surface-2)] px-1.5 py-0.5 font-mono text-[13px] text-[var(--ink)]",
          className,
        )}
        {...props}
      >
        {children}
      </code>
    )
  },
  pre: ({ className, ...props }) => (
    <pre
      className={cn(
        "overflow-x-auto border border-[var(--border)] bg-[var(--surface-2)] p-3 leading-relaxed",
        className,
      )}
      {...props}
    />
  ),
}

// CommonMark (which remark-gfm builds on) only recognizes a table if a
// blank line precedes it. Models routinely skip that blank line when a
// table follows a sentence directly — without this, the header row gets
// absorbed into the preceding paragraph and only the "|---|---|"
// separator line renders, as literal text instead of a table. This finds
// exactly that shape (a row with a pipe, immediately followed by a
// separator row, immediately preceded by non-blank text) and inserts the
// blank line CommonMark needs.
const SEPARATOR_ROW = /^[\s|:-]+$/

function looksLikeSeparatorRow(line: string) {
  return SEPARATOR_ROW.test(line) && line.includes("-")
}

function ensureBlankLineBeforeTables(markdown: string): string {
  const lines = markdown.split("\n")
  const out: string[] = []
  for (let i = 0; i < lines.length; i++) {
    const line = lines[i]
    const next = lines[i + 1]
    const prevOut = out[out.length - 1]
    const startsTable =
      line.includes("|") && next !== undefined && looksLikeSeparatorRow(next)
    if (startsTable && prevOut !== undefined && prevOut.trim() !== "") {
      out.push("")
    }
    out.push(line)
  }
  return out.join("\n")
}

// GFM tables use the SEPARATOR row's column count as authoritative — if
// the model writes a 3-column header ("| Column | Range | AVG |") but a
// 2-column separator ("|---|---|"), the third column doesn't error, it
// just silently disappears from every row. Rather than trust the model
// to keep every row's column count matching (it often doesn't), this
// walks each detected table block and pads every row — header,
// separator, and data — out to the widest row actually present, so no
// column gets dropped.
function splitTableRow(line: string): string[] {
  let s = line.trim()
  if (s.startsWith("|")) s = s.slice(1)
  if (s.endsWith("|")) s = s.slice(0, -1)
  return s.split("|").map((cell) => cell.trim())
}

function buildTableRow(cells: string[]): string {
  return `| ${cells.join(" | ")} |`
}

function normalizeTableColumns(markdown: string): string {
  const lines = markdown.split("\n")
  const out: string[] = []
  let i = 0

  while (i < lines.length) {
    const line = lines[i]
    const next = lines[i + 1]

    if (line.includes("|") && next !== undefined && looksLikeSeparatorRow(next)) {
      const headerCells = splitTableRow(line)
      const separatorCells = splitTableRow(next)
      const dataRows: string[][] = []

      let j = i + 2
      while (j < lines.length && lines[j].includes("|") && lines[j].trim() !== "") {
        dataRows.push(splitTableRow(lines[j]))
        j++
      }

      const width = Math.max(
        headerCells.length,
        separatorCells.length,
        ...dataRows.map((row) => row.length),
      )
      const padTo = (cells: string[], fill: string) =>
        cells.length >= width ? cells : [...cells, ...Array(width - cells.length).fill(fill)]

      out.push(buildTableRow(padTo(headerCells, "")))
      out.push(
        buildTableRow(
          padTo(
            separatorCells.map((c) => (/^:?-+:?$/.test(c) ? c : "---")),
            "---",
          ),
        ),
      )
      for (const row of dataRows) out.push(buildTableRow(padTo(row, "")))

      i = j
      continue
    }

    out.push(line)
    i++
  }

  return out.join("\n")
}

function normalizeMarkdown(markdown: string): string {
  return normalizeTableColumns(ensureBlankLineBeforeTables(markdown))
}

function Markdown({ children }: { children: string }) {
  return (
    <div className="flex flex-col gap-2.5 text-[14px]">
      <ReactMarkdown remarkPlugins={[remarkGfm]} components={components}>
        {normalizeMarkdown(children)}
      </ReactMarkdown>
    </div>
  )
}

export { Markdown }
