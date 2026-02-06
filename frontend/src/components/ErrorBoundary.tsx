import { Component, type ReactNode } from 'react'
import { AlertTriangle } from 'lucide-react'
import { Button } from './ui/Button'

interface ErrorBoundaryProps {
  children: ReactNode
}

interface ErrorBoundaryState {
  hasError: boolean
  error: Error | null
}

/**
 * Global error boundary that catches rendering errors
 * Displays full-page error state with reload option
 */
export class ErrorBoundary extends Component<ErrorBoundaryProps, ErrorBoundaryState> {
  constructor(props: ErrorBoundaryProps) {
    super(props)
    this.state = { hasError: false, error: null }
  }

  static getDerivedStateFromError(error: Error): ErrorBoundaryState {
    return { hasError: true, error }
  }

  componentDidCatch(error: Error, errorInfo: React.ErrorInfo) {
    console.error('ErrorBoundary caught an error:', error, errorInfo)
  }

  handleReload = () => {
    window.location.reload()
  }

  render() {
    if (this.state.hasError) {
      return (
        <div className="flex items-center justify-center min-h-screen bg-surface-secondary px-4">
          <div className="text-center max-w-md">
            <AlertTriangle className="w-16 h-16 mx-auto mb-6 text-red-600" />
            <h1 className="text-2xl font-bold text-text-primary mb-3">
              Something unexpected happened
            </h1>
            <p className="text-base text-text-secondary mb-6">
              We encountered an error while rendering this page. Reloading may fix the issue.
            </p>
            {import.meta.env.DEV && this.state.error && (
              <div className="mb-6 p-4 bg-red-50 border border-red-200 rounded text-left">
                <p className="text-xs font-mono text-red-800 break-all">
                  {this.state.error.message}
                </p>
              </div>
            )}
            <Button variant="primary" onClick={this.handleReload}>
              Reload page
            </Button>
          </div>
        </div>
      )
    }

    return this.props.children
  }
}
