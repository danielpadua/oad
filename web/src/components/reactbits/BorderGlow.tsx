import { type CSSProperties, type ReactNode } from "react";
import { cn } from "@/lib/utils";

interface BorderGlowProps {
  children: ReactNode;
  className?: string;
  color?: string;
  size?: number;
}

export function BorderGlow({
  children,
  className,
  color = "#818cf8",
  size = 6,
}: BorderGlowProps) {
  const style: CSSProperties = {
    boxShadow: `0 0 ${size}px ${color}, 0 0 ${size * 2}px ${color}40`,
    borderColor: `${color}90`,
    animation: "border-glow-pulse 2s ease-in-out infinite",
  };

  return (
    <>
      <style>{`
        @keyframes border-glow-pulse {
          0%, 100% { opacity: 0.85; }
          50% { opacity: 1; }
        }
      `}</style>
      <div className={cn("rounded-lg border", className)} style={style}>
        {children}
      </div>
    </>
  );
}
