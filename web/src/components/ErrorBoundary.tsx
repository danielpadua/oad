import * as React from "react"
import { AlertTriangle } from "lucide-react"
import { Button } from "@/components/ui/button"

interface ErrorBoundaryProps {
  children: React.ReactNode
  fallback?: React.ReactNode
  onError?: (error: Error, info: React.ErrorInfo) => void
}

interface ErrorBoundaryState {
  error: Error | null
}

class ErrorBoundary extends React.Component<ErrorBoundaryProps, ErrorBoundaryState> {
  constructor(props: ErrorBoundaryProps) {
    super(props)
    this.state = { error: null }
  }

  static getDerivedStateFromError(error: Error): ErrorBoundaryState {
    return { error }
  }

  componentDidCatch(error: Error, info: React.ErrorInfo) {
    this.props.onError?.(error, info)
    console.error("[ErrorBoundary]", error, info)
  }

  reset = () => this.setState({ error: null })

  render() {
    const { error } = this.state
    const { children, fallback } = this.props

    if (error) {
      if (fallback) return fallback

      return (
        <div className="flex flex-col items-center justify-center gap-4 rounded-lg border border-destructive/30 bg-destructive/5 p-10 text-center">
          <div className="flex size-12 items-center justify-center rounded-full bg-destructive/10">
            <AlertTriangle className="size-5 text-destructive" />
          </div>
          <div className="space-y-1">
            <p className="text-sm font-medium text-foreground">
              Something went wrong
            </p>
            <p className="max-w-xs text-sm text-muted-foreground">
              {error.message}
            </p>
          </div>
          <Button variant="outline" size="sm" onClick={this.reset}>
            Try again
          </Button>
        </div>
      )
    }

    return children
  }
}

export { ErrorBoundary }
