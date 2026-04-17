import { type ReactNode, useRef } from "react";
import { motion, useInView, type Variants } from "framer-motion";
import { cn } from "@/lib/utils";

interface AnimatedContentProps {
  children: ReactNode;
  className?: string;
  distance?: number;
  direction?: "up" | "down" | "left" | "right";
  delay?: number;
  duration?: number;
  once?: boolean;
  threshold?: number;
}

export function AnimatedContent({
  children,
  className,
  distance = 40,
  direction = "up",
  delay = 0,
  duration = 0.5,
  once = true,
  threshold = 0.1,
}: AnimatedContentProps) {
  const ref = useRef<HTMLDivElement>(null);
  const isInView = useInView(ref, { once, amount: threshold });

  const directionMap = {
    up: { y: distance },
    down: { y: -distance },
    left: { x: distance },
    right: { x: -distance },
  };

  const variants: Variants = {
    hidden: {
      opacity: 0,
      ...directionMap[direction],
    },
    visible: {
      opacity: 1,
      x: 0,
      y: 0,
      transition: {
        duration,
        delay,
        ease: [0.25, 0.46, 0.45, 0.94],
      },
    },
  };

  return (
    <motion.div
      ref={ref}
      className={cn(className)}
      initial="hidden"
      animate={isInView ? "visible" : "hidden"}
      variants={variants}
    >
      {children}
    </motion.div>
  );
}
