import { Check } from "lucide-react"
import { cn } from "@/lib/utils"

export interface StepConfig {
  title: string
  description?: string
}

export interface StepperProps {
  steps: StepConfig[]
  currentStep: number
  className?: string
}

export function Stepper({ steps, currentStep, className }: StepperProps) {
  return (
    <div className={cn("flex flex-col", className)}>
      {steps.map((step, index) => {
        const isCompleted = index < currentStep
        const isActive = index === currentStep
        const isLast = index === steps.length - 1

        return (
          <div key={index} className="flex items-start gap-3">
            <div className="flex flex-col items-center">
              <div
                className={cn(
                  "flex size-7 shrink-0 items-center justify-center rounded-full border-2 text-xs font-semibold transition-all duration-200",
                  isCompleted && "border-primary bg-primary text-primary-foreground",
                  isActive &&
                    "border-primary bg-background text-primary ring-4 ring-primary/10",
                  !isCompleted &&
                    !isActive &&
                    "border-border bg-background text-muted-foreground"
                )}
              >
                {isCompleted ? (
                  <Check className="size-3.5" />
                ) : (
                  <span>{index + 1}</span>
                )}
              </div>
              {!isLast && (
                <div
                  className={cn(
                    "mt-1 w-px min-h-8 flex-1 transition-colors duration-300",
                    index < currentStep ? "bg-primary" : "bg-border"
                  )}
                />
              )}
            </div>
            <div className={cn("pt-0.5 pb-6", isLast && "pb-0")}>
              <p
                className={cn(
                  "text-sm font-medium leading-6",
                  isActive || isCompleted
                    ? "text-foreground"
                    : "text-muted-foreground"
                )}
              >
                {step.title}
              </p>
              {step.description && (
                <p className="mt-0.5 text-xs leading-relaxed text-muted-foreground">
                  {step.description}
                </p>
              )}
            </div>
          </div>
        )
      })}
    </div>
  )
}
