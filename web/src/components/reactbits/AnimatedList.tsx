import { type ReactNode } from "react"
import { AnimatePresence, motion } from "framer-motion"
import { cn } from "@/lib/utils"

interface AnimatedListProps {
  children: ReactNode[]
  className?: string
  itemClassName?: string
  delay?: number
  duration?: number
  gap?: string
}

export function AnimatedList({
  children,
  className,
  itemClassName,
  delay = 0.05,
  duration = 0.35,
  gap,
}: AnimatedListProps) {
  return (
    <ul
      className={cn("flex flex-col", className)}
      style={gap ? { gap } : undefined}
    >
      <AnimatePresence initial={false}>
        {children.map((child, i) => (
          <motion.li
            key={i}
            className={cn(itemClassName)}
            initial={{ opacity: 0, y: -12, scale: 0.97 }}
            animate={{ opacity: 1, y: 0, scale: 1 }}
            exit={{ opacity: 0, scale: 0.97, transition: { duration: 0.15 } }}
            transition={{
              duration,
              delay: i * delay,
              ease: [0.25, 0.46, 0.45, 0.94],
            }}
          >
            {child}
          </motion.li>
        ))}
      </AnimatePresence>
    </ul>
  )
}
