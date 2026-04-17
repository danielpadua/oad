import { cn } from "@/lib/utils";

interface ShinyTextProps {
  text: string;
  className?: string;
  speed?: number;
  colors?: string[];
}

export function ShinyText({
  text,
  className,
  speed = 4,
  colors = ["#818cf8", "#c4b5fd", "#e0e7ff", "#c4b5fd", "#818cf8"],
}: ShinyTextProps) {
  return (
    <>
      <style>{`
        @keyframes shiny-text {
          0%   { background-position: 200% center; }
          100% { background-position: -200% center; }
        }
      `}</style>
      <span
        className={cn("inline-block font-medium", className)}
        style={{
          backgroundImage: `linear-gradient(90deg, ${colors.join(", ")})`,
          backgroundSize: "200% auto",
          WebkitBackgroundClip: "text",
          WebkitTextFillColor: "transparent",
          backgroundClip: "text",
          animation: `shiny-text ${speed}s linear infinite`,
        }}
      >
        {text}
      </span>
    </>
  );
}
