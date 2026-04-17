import { type ReactNode, useRef } from "react";
import { motion, useMotionValue, useSpring, useTransform } from "framer-motion";
import { cn } from "@/lib/utils";

interface DockItemProps {
  children: ReactNode;
  className?: string;
  mousePos: ReturnType<typeof useMotionValue<number>>;
  axis: "x" | "y";
  magnification?: number;
  distance?: number;
}

function DockItem({
  children,
  className,
  mousePos,
  axis,
  magnification = 60,
  distance = 120,
}: DockItemProps) {
  const ref = useRef<HTMLDivElement>(null);

  const distanceCalc = useTransform(mousePos, (val: number) => {
    const rect = ref.current?.getBoundingClientRect();
    if (!rect) return Infinity;
    const center = axis === "y" ? rect.top + rect.height / 2 : rect.left + rect.width / 2;
    return val - center;
  });

  const sizeTransform = useTransform(
    distanceCalc,
    [-distance, 0, distance],
    [40, magnification, 40]
  );
  const size = useSpring(sizeTransform, { mass: 0.1, stiffness: 180, damping: 18 });

  return (
    <motion.div
      ref={ref}
      style={{ width: size, height: size }}
      className={cn("flex items-center justify-center", className)}
    >
      {children}
    </motion.div>
  );
}

interface DockProps {
  children: ReactNode;
  className?: string;
  orientation?: "horizontal" | "vertical";
  magnification?: number;
  distance?: number;
}

export function Dock({
  children,
  className,
  orientation = "vertical",
  magnification = 56,
  distance = 120,
}: DockProps) {
  const mousePos = useMotionValue(Infinity);
  const axis = orientation === "vertical" ? "y" : "x";

  return (
    <motion.div
      onMouseMove={(e) => mousePos.set(axis === "y" ? e.pageY : e.pageX)}
      onMouseLeave={() => mousePos.set(Infinity)}
      className={cn(
        "flex items-center",
        orientation === "vertical" ? "flex-col" : "flex-row",
        className
      )}
    >
      {Array.isArray(children)
        ? children.map((child, i) => (
            <DockItem key={i} mousePos={mousePos} axis={axis} magnification={magnification} distance={distance}>
              {child}
            </DockItem>
          ))
        : children}
    </motion.div>
  );
}
