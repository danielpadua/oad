import * as React from "react"
import {
  Sheet,
  SheetContent,
  SheetDescription,
  SheetFooter,
  SheetHeader,
  SheetTitle,
} from "@/components/ui/sheet"

export interface DrawerProps {
  open: boolean
  onOpenChange: (open: boolean) => void
  title?: React.ReactNode
  description?: React.ReactNode
  children: React.ReactNode
  footer?: React.ReactNode
  side?: "left" | "right" | "top" | "bottom"
  width?: "sm" | "md" | "lg" | "xl"
}

const widthClasses: Record<NonNullable<DrawerProps["width"]>, string> = {
  sm: "w-80",
  md: "w-96",
  lg: "w-[32rem]",
  xl: "w-[42rem]",
}

function Drawer({
  open,
  onOpenChange,
  title,
  description,
  children,
  footer,
  side = "right",
  width = "md",
}: DrawerProps) {
  return (
    <Sheet open={open} onOpenChange={onOpenChange}>
      <SheetContent
        side={side}
        className={
          side === "left" || side === "right"
            ? widthClasses[width]
            : undefined
        }
      >
        {(title || description) && (
          <SheetHeader>
            {title && <SheetTitle>{title}</SheetTitle>}
            {description && <SheetDescription>{description}</SheetDescription>}
          </SheetHeader>
        )}
        <div className="flex-1 overflow-y-auto py-4">{children}</div>
        {footer && <SheetFooter>{footer}</SheetFooter>}
      </SheetContent>
    </Sheet>
  )
}

export { Drawer }
