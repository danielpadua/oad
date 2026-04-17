import { useEffect, useRef, useState } from "react";
import { useInView } from "framer-motion";
import { cn } from "@/lib/utils";

interface CountUpProps {
  from?: number;
  to: number;
  duration?: number;
  delay?: number;
  decimals?: number;
  separator?: string;
  className?: string;
  once?: boolean;
  onComplete?: () => void;
}

function easeOutQuart(t: number): number {
  return 1 - Math.pow(1 - t, 4);
}

export function CountUp({
  from = 0,
  to,
  duration = 2,
  delay = 0,
  decimals = 0,
  separator = ",",
  className,
  once = true,
  onComplete,
}: CountUpProps) {
  const [value, setValue] = useState(from);
  const ref = useRef<HTMLSpanElement>(null);
  const isInView = useInView(ref, { once, amount: 0.5 });
  const animationRef = useRef<number | null>(null);

  useEffect(() => {
    if (!isInView) return;

    // Cancel any running animation before starting a new one.
    if (animationRef.current !== null) {
      cancelAnimationFrame(animationRef.current);
      animationRef.current = null;
    }

    const startValue = from;
    const timer = setTimeout(() => {
      const startTime = performance.now();
      const totalDuration = duration * 1000;

      const animate = (now: number) => {
        const elapsed = now - startTime;
        const progress = Math.min(elapsed / totalDuration, 1);
        const current = startValue + (to - startValue) * easeOutQuart(progress);

        setValue(current);

        if (progress < 1) {
          animationRef.current = requestAnimationFrame(animate);
        } else {
          setValue(to);
          animationRef.current = null;
          onComplete?.();
        }
      };

      animationRef.current = requestAnimationFrame(animate);
    }, delay * 1000);

    return () => {
      clearTimeout(timer);
      if (animationRef.current !== null) {
        cancelAnimationFrame(animationRef.current);
        animationRef.current = null;
      }
    };
  }, [isInView, from, to, duration, delay, onComplete]);

  const formatted = value
    .toFixed(decimals)
    .replace(/\B(?=(\d{3})+(?!\d))/g, separator);

  return (
    <span ref={ref} className={cn("tabular-nums", className)}>
      {formatted}
    </span>
  );
}
