import { type CSSProperties, type ReactNode } from "react";
import { cn } from "@/lib/utils";

interface GradientTextProps {
  children: ReactNode;
  className?: string;
  colors?: string[];
  animationSpeed?: number;
}

export function GradientText({
  children,
  className,
  colors = ["#ffaa40", "#9b59b6", "#4facfe", "#00f2fe"],
  animationSpeed = 8,
}: GradientTextProps) {
  const style: CSSProperties = {
    backgroundImage: `linear-gradient(to right, ${colors.join(", ")}, ${colors[0]})`,
    backgroundSize: "300% 100%",
    WebkitBackgroundClip: "text",
    WebkitTextFillColor: "transparent",
    backgroundClip: "text",
    animation: `gradient-shift ${animationSpeed}s linear infinite`,
  };

  return (
    <>
      <style>{`
        @keyframes gradient-shift {
          0%   { background-position: 0% 50%; }
          100% { background-position: 300% 50%; }
        }
      `}</style>
      <span className={cn("inline-block font-semibold", className)} style={style}>
        {children}
      </span>
    </>
  );
}
