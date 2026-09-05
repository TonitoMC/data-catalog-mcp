import { cva, type VariantProps } from "class-variance-authority"
import * as React from "react"

import { cn } from "@/lib/utils"

const buttonVariants = cva(
  "inline-flex items-center justify-center gap-1.5 whitespace-nowrap border font-mono text-[13px] font-medium tracking-tight transition-colors focus-visible:outline-2 focus-visible:outline-offset-1 focus-visible:outline-[var(--accent)] disabled:pointer-events-none disabled:opacity-50",
  {
    variants: {
      variant: {
        primary:
          "border-[var(--accent)] bg-[var(--accent)] text-[var(--accent-ink)] hover:brightness-105 active:brightness-95",
        outline:
          "border-[var(--border-strong)] bg-transparent text-[var(--ink)] hover:border-[var(--accent)] hover:text-[var(--accent)]",
        ghost:
          "border-transparent bg-transparent text-[var(--ink-soft)] hover:border-[var(--border)] hover:text-[var(--ink)]",
      },
      size: {
        sm: "h-7 px-2.5 text-xs",
        default: "h-9 px-3.5",
      },
    },
    defaultVariants: {
      variant: "outline",
      size: "default",
    },
  },
)

interface ButtonProps
  extends React.ButtonHTMLAttributes<HTMLButtonElement>,
    VariantProps<typeof buttonVariants> {}

function Button({ className, variant, size, ...props }: ButtonProps) {
  return (
    <button className={cn(buttonVariants({ variant, size }), className)} {...props} />
  )
}

export { Button, buttonVariants }
