function Thinking() {
  return (
    <div className="flex items-center gap-2">
      <span className="flex items-center gap-1">
        <span className="size-1.5 animate-bounce rounded-full bg-[var(--ink-soft)] [animation-delay:-0.3s]" />
        <span className="size-1.5 animate-bounce rounded-full bg-[var(--ink-soft)] [animation-delay:-0.15s]" />
        <span className="size-1.5 animate-bounce rounded-full bg-[var(--ink-soft)]" />
      </span>
      <span className="font-mono text-[11px] text-[var(--ink-soft)]">thinking…</span>
    </div>
  )
}

export { Thinking }
