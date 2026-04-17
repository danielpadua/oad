import { GradientText } from "@/components/reactbits";

export function TopBar() {
  return (
    <header className="flex h-14 items-center border-b border-border bg-card px-6">
      <GradientText
        className="text-xl tracking-tight"
        colors={["#818cf8", "#a78bfa", "#38bdf8", "#818cf8"]}
        animationSpeed={6}
      >
        OAD
      </GradientText>
      <span className="ml-2 text-sm text-muted-foreground">
        Open Authoritative Directory
      </span>
    </header>
  );
}
