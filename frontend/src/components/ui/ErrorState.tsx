import { AlertTriangle, WifiOff } from 'lucide-react'
import { Button } from './Button'

interface ErrorStateProps {
  variant?: 'default' | 'not-found' | 'network'
  title?: string
  message: string
  onRetry?: () => void
  retryLabel?: string
}

/**
 * Reusable error state component for failed operations
 * Variants: default (general errors), not-found (404), network (connection issues)
 * Centered with red alert icon and retry/action button
 */
export function ErrorState({
  variant = 'default',
  title,
  message,
  onRetry,
  retryLabel = 'Try again',
}: ErrorStateProps) {
  const getIcon = () => {
    switch (variant) {
      case 'network':
        return <WifiOff className="w-12 h-12 mx-auto mb-4 text-red-600" />
      default:
        return <AlertTriangle className="w-12 h-12 mx-auto mb-4 text-red-600" />
    }
  }

  const getDefaultTitle = () => {
    switch (variant) {
      case 'not-found':
        return 'Incident not found'
      case 'network':
        return 'Connection lost'
      default:
        return 'Something went wrong'
    }
  }

  return (
    <div className="flex items-center justify-center min-h-[400px] px-4">
      <div className="text-center max-w-md">
        {getIcon()}
        <h3 className="text-lg font-semibold text-text-primary mb-2">
          {title || getDefaultTitle()}
        </h3>
        <p className="text-sm text-text-secondary mb-6">{message}</p>
        {onRetry && (
          <Button variant="primary" onClick={onRetry}>
            {retryLabel}
          </Button>
        )}
      </div>
    </div>
  )
}

/**
 * Pre-configured error state for 404 not found
 */
export function NotFoundError({ onGoBack }: { onGoBack?: () => void }) {
  return (
    <ErrorState
      variant="not-found"
      message="The incident you are looking for does not exist."
      onRetry={onGoBack}
      retryLabel="Go to incidents"
    />
  )
}

/**
 * Pre-configured error state for network errors
 */
export function NetworkError({ onRetry }: { onRetry: () => void }) {
  return (
    <ErrorState
      variant="network"
      message="Unable to reach the server. Please check your connection and try again."
      onRetry={onRetry}
      retryLabel="Retry"
    />
  )
}

/**
 * Pre-configured error state for general errors
 */
export function GeneralError({ message, onRetry }: { message: string; onRetry?: () => void }) {
  return <ErrorState variant="default" message={message} onRetry={onRetry} />
}
