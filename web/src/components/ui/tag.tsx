import * as React from "react"
import { X } from "lucide-react"
import { cva, type VariantProps } from "class-variance-authority"
import { cn } from "@/lib/utils"

const tagVariants = cva(
  "inline-flex items-center gap-1 rounded-md border px-2 py-0.5 text-xs font-medium transition-colors focus:outline-none focus:ring-2 focus:ring-ring focus:ring-offset-2",
  {
    variants: {
      variant: {
        default:
          "border-transparent bg-primary/10 text-primary hover:bg-primary/20",
        secondary:
          "border-transparent bg-secondary text-secondary-foreground hover:bg-secondary/80",
        destructive:
          "border-transparent bg-destructive/10 text-destructive hover:bg-destructive/20",
        outline: "border-border text-foreground",
      },
    },
    defaultVariants: {
      variant: "default",
    },
  }
)

export interface TagProps
  extends React.HTMLAttributes<HTMLSpanElement>,
    VariantProps<typeof tagVariants> {
  onRemove?: () => void
  removable?: boolean
}

function Tag({
  className,
  variant,
  children,
  onRemove,
  removable = false,
  ...props
}: TagProps) {
  return (
    <span
      data-slot="tag"
      className={cn(tagVariants({ variant }), className)}
      {...props}
    >
      {children}
      {(removable || onRemove) && (
        <button
          type="button"
          onClick={(e) => {
            e.stopPropagation()
            onRemove?.()
          }}
          className="ml-0.5 rounded-sm opacity-60 ring-offset-background transition-opacity hover:opacity-100 focus:outline-none focus:ring-2 focus:ring-ring focus:ring-offset-2"
          aria-label="Remove tag"
        >
          <X className="size-3" />
        </button>
      )}
    </span>
  )
}

export { Tag, tagVariants }
