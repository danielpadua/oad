import { type ReactNode, useRef } from "react";
import { motion, useInView } from "framer-motion";
import { cn } from "@/lib/utils";

interface FadeContentProps {
  children: ReactNode;
  className?: string;
  blur?: boolean;
  duration?: number;
  delay?: number;
  threshold?: number;
  once?: boolean;
}

export function FadeContent({
  children,
  className,
  blur = false,
  duration = 0.4,
  delay = 0,
  threshold = 0.1,
  once = true,
}: FadeContentProps) {
  const ref = useRef<HTMLDivElement>(null);
  const isInView = useInView(ref, { once, amount: threshold });

  return (
    <motion.div
      ref={ref}
      className={cn(className)}
      initial={{ opacity: 0, filter: blur ? "blur(8px)" : "none" }}
      animate={
        isInView
          ? { opacity: 1, filter: "blur(0px)" }
          : { opacity: 0, filter: blur ? "blur(8px)" : "none" }
      }
      transition={{ duration, delay, ease: "easeOut" }}
    >
      {children}
    </motion.div>
  );
}
