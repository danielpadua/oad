import { useEffect, useRef, useState } from "react";
import { cn } from "@/lib/utils";

const CHARSET = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789@#$%&*!?";

function randomChar() {
  return CHARSET[Math.floor(Math.random() * CHARSET.length)];
}

interface DecryptedTextProps {
  text: string;
  className?: string;
  /** Total animation duration in ms. */
  duration?: number;
  /** Delay before the animation starts in ms. */
  delay?: number;
}

export function DecryptedText({ text, className, duration = 1600, delay = 0 }: DecryptedTextProps) {
  const chars = text.split("");
  const [displayed, setDisplayed] = useState<string[]>(() => chars.map((c) => (c === " " ? " " : randomChar())));
  const rafRef = useRef<number>(0);
  const startRef = useRef<number | null>(null);

  useEffect(() => {
    const timeout = setTimeout(() => {
      function tick(timestamp: number) {
        if (startRef.current === null) startRef.current = timestamp;
        const progress = Math.min((timestamp - startRef.current) / duration, 1);
        const revealCount = Math.floor(progress * chars.length);

        setDisplayed(
          chars.map((char, i) => {
            if (char === " ") return " ";
            return i < revealCount ? char : randomChar();
          }),
        );

        if (progress < 1) {
          rafRef.current = requestAnimationFrame(tick);
        }
      }

      rafRef.current = requestAnimationFrame(tick);
    }, delay);

    return () => {
      clearTimeout(timeout);
      cancelAnimationFrame(rafRef.current);
    };
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [text, duration, delay]);

  return (
    <span className={cn("font-mono", className)} aria-label={text}>
      {displayed.map((char, i) => (
        <span
          key={i}
          className={
            char !== " " && char !== chars[i]
              ? "text-primary/50 transition-colors duration-75"
              : "transition-colors duration-75"
          }
        >
          {char}
        </span>
      ))}
    </span>
  );
}
