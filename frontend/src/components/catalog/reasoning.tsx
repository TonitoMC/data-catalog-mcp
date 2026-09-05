// Rendered inline with the rest of the answer, not a separate collapsible
// panel — just visually de-emphasized so it reads as "the model thinking
// out loud" rather than the answer itself. Plain text (no markdown parse):
// reasoning content is prose, not a place tables/code blocks are expected.
function Reasoning({ text }: { text: string }) {
  return (
    <p className="leading-relaxed text-[var(--ink-soft)] italic">{text}</p>
  )
}

export { Reasoning }
