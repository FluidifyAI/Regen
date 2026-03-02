import { Sparkles, RefreshCw } from 'lucide-react'

interface AIButtonProps {
  onClick: () => void
  loading?: boolean
  disabled?: boolean
  children: React.ReactNode
  className?: string
}

/**
 * AI action button — solid brand blue at rest, violet→blue→cyan gradient on hover.
 * The gradient fades in via an overlay so the transition works across all browsers.
 */
export function AIButton({ onClick, loading, disabled, children, className = '' }: AIButtonProps) {
  return (
    <button
      type="button"
      onClick={onClick}
      disabled={disabled || loading}
      className={`relative group inline-flex items-center gap-1 px-2.5 py-1 rounded text-white text-xs font-medium bg-brand-primary overflow-hidden transition-all disabled:opacity-50 disabled:cursor-not-allowed ${className}`}
    >
      {/* Gradient overlay — fades in on hover */}
      <span className="absolute inset-0 bg-gradient-to-r from-violet-500 via-blue-500 to-cyan-400 opacity-0 group-hover:opacity-100 group-disabled:opacity-0 transition-opacity duration-300 ease-in-out" />
      <span className="relative z-10 flex items-center gap-1">
        {loading
          ? <RefreshCw className="w-3 h-3 animate-spin" />
          : <Sparkles className="w-3 h-3" />
        }
        {children}
      </span>
    </button>
  )
}
