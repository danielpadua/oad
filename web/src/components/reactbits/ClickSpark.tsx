import { useState, useRef, type ReactNode } from "react"
import { AnimatePresence, motion } from "framer-motion"

interface Spark {
  id: number
  x: number
  y: number
}

interface ClickSparkProps {
  children: ReactNode
  count?: number
  spread?: number
  size?: number
  duration?: number
  color?: string
  disabled?: boolean
}

let _id = 0

export function ClickSpark({
  children,
  count = 8,
  spread = 36,
  size = 5,
  duration = 0.45,
  color = "currentColor",
  disabled = false,
}: ClickSparkProps) {
  const containerRef = useRef<HTMLDivElement>(null)
  const [sparks, setSparks] = useState<Spark[]>([])

  function handleClick(e: React.MouseEvent<HTMLDivElement>) {
    if (disabled) return
    const rect = containerRef.current?.getBoundingClientRect()
    if (!rect) return

    const x = e.clientX - rect.left
    const y = e.clientY - rect.top
    const id = _id++

    setSparks((prev) => [...prev, { id, x, y }])

    setTimeout(() => {
      setSparks((prev) => prev.filter((s) => s.id !== id))
    }, duration * 1000 + 100)
  }

  return (
    <div ref={containerRef} className="relative inline-block" onClickCapture={handleClick}>
      {children}
      <AnimatePresence>
        {sparks.flatMap((spark) =>
          Array.from({ length: count }, (_, i) => {
            const angle = (i / count) * 2 * Math.PI
            const dx = Math.cos(angle) * spread
            const dy = Math.sin(angle) * spread

            return (
              <motion.span
                key={`${spark.id}-${i}`}
                initial={{ opacity: 1, scale: 1, x: spark.x, y: spark.y }}
                animate={{ opacity: 0, scale: 0, x: spark.x + dx, y: spark.y + dy }}
                exit={{}}
                transition={{ duration, ease: "easeOut" }}
                style={{
                  position: "absolute",
                  top: 0,
                  left: 0,
                  width: size,
                  height: size,
                  borderRadius: "50%",
                  backgroundColor: color,
                  pointerEvents: "none",
                  translateX: "-50%",
                  translateY: "-50%",
                }}
              />
            )
          })
        )}
      </AnimatePresence>
    </div>
  )
}
