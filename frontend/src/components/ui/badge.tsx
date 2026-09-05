import { cva, type VariantProps } from "class-variance-authority"
import * as React from "react"

import { cn } from "@/lib/utils"

const badgeVariants = cva(
  "inline-flex items-center gap-1 border px-1.5 py-0.5 font-mono text-[10.5px] font-medium tracking-[0.04em] uppercase",
  {
    variants: {
      variant: {
        neutral: "border-[var(--border)] bg-[var(--surface-2)] text-[var(--ink-soft)]",
        accent: "border-[var(--accent)] bg-[var(--accent-soft)] text-[var(--accent)]",
        ok: "border-[var(--ok)] bg-[var(--ok-soft)] text-[var(--ok)]",
        secondary:
          "border-[var(--secondary)] bg-[var(--secondary-soft)] text-[var(--secondary)]",
      },
    },
    defaultVariants: { variant: "neutral" },
  },
)

interface BadgeProps
  extends React.ComponentProps<"span">,
    VariantProps<typeof badgeVariants> {}

function Badge({ className, variant, ...props }: BadgeProps) {
  return <span className={cn(badgeVariants({ variant }), className)} {...props} />
}

export { Badge, badgeVariants }
