import { useRef } from "react"
import { motion, useInView, type Variants } from "framer-motion"
import { cn } from "@/lib/utils"

interface BlurTextProps {
  text: string
  className?: string
  delay?: number
  duration?: number
  wordDelay?: number
  once?: boolean
}

export function BlurText({
  text,
  className,
  delay = 0,
  duration = 0.4,
  wordDelay = 0.06,
  once = true,
}: BlurTextProps) {
  const ref = useRef<HTMLSpanElement>(null)
  const isInView = useInView(ref, { once })

  const words = text.split(" ")

  const containerVariants: Variants = {
    hidden: {},
    visible: {
      transition: {
        staggerChildren: wordDelay,
        delayChildren: delay,
      },
    },
  }

  const wordVariants: Variants = {
    hidden: { opacity: 0, filter: "blur(8px)", y: 4 },
    visible: {
      opacity: 1,
      filter: "blur(0px)",
      y: 0,
      transition: { duration, ease: "easeOut" },
    },
  }

  return (
    <motion.span
      ref={ref}
      className={cn("inline-flex flex-wrap gap-x-[0.25em]", className)}
      variants={containerVariants}
      initial="hidden"
      animate={isInView ? "visible" : "hidden"}
    >
      {words.map((word, i) => (
        <motion.span key={i} variants={wordVariants} className="inline-block">
          {word}
        </motion.span>
      ))}
    </motion.span>
  )
}
