import { motion } from "framer-motion";
import { cn } from "@/lib/utils";

interface Blob {
  color: string;
  left: string;
  top: string;
  initialScale: number;
  duration: number;
}

interface SoftAuroraProps {
  className?: string;
  colors?: [string, string, string, string];
}

const defaultColors: [string, string, string, string] = ["#3b82f6", "#8b5cf6", "#06b6d4", "#10b981"];

export function SoftAurora({ className, colors = defaultColors }: SoftAuroraProps) {
  const blobs: Blob[] = [
    { color: colors[0], left: "15%", top: "25%", initialScale: 1.1, duration: 9 },
    { color: colors[1], left: "72%", top: "18%", initialScale: 0.9, duration: 13 },
    { color: colors[2], left: "58%", top: "68%", initialScale: 1.2, duration: 11 },
    { color: colors[3], left: "10%", top: "72%", initialScale: 0.85, duration: 15 },
  ];

  return (
    <div className={cn("pointer-events-none absolute inset-0 overflow-hidden", className)}>
      {blobs.map((blob, i) => (
        <motion.div
          key={i}
          className="absolute -translate-x-1/2 -translate-y-1/2 rounded-full blur-3xl"
          style={{
            width: "45%",
            height: "45%",
            background: blob.color,
            left: blob.left,
            top: blob.top,
            opacity: 0.22,
          }}
          animate={{
            scale: [blob.initialScale, blob.initialScale * 1.18, blob.initialScale * 0.92, blob.initialScale],
            x: [0, 28, -18, 8, 0],
            y: [0, -22, 20, -8, 0],
          }}
          transition={{
            duration: blob.duration,
            repeat: Infinity,
            ease: "easeInOut",
            times: [0, 0.3, 0.65, 1],
          }}
        />
      ))}
    </div>
  );
}
