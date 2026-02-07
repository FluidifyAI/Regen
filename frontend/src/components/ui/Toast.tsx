import { useEffect, useState } from 'react'
import { CheckCircle, XCircle, AlertCircle, X } from 'lucide-react'

export type ToastType = 'success' | 'error' | 'info'

interface Toast {
  id: string
  type: ToastType
  message: string
  duration?: number
}

interface ToastProps {
  toast: Toast
  onDismiss: (id: string) => void
}

/**
 * Toast notification component
 * Auto-dismisses after duration (default 5s)
 */
export function Toast({ toast, onDismiss }: ToastProps) {
  const [isExiting, setIsExiting] = useState(false)

  useEffect(() => {
    const duration = toast.duration || 5000
    const timer = setTimeout(() => {
      setIsExiting(true)
      setTimeout(() => onDismiss(toast.id), 300) // Wait for exit animation
    }, duration)

    return () => clearTimeout(timer)
  }, [toast.id, toast.duration, onDismiss])

  const handleDismiss = () => {
    setIsExiting(true)
    setTimeout(() => onDismiss(toast.id), 300)
  }

  const { icon: Icon, bgColor, textColor, borderColor } = getToastStyles(toast.type)

  return (
    <div
      className={`flex items-start gap-3 p-4 rounded-lg border shadow-lg max-w-md transition-all duration-300 ${
        isExiting ? 'opacity-0 translate-x-full' : 'opacity-100 translate-x-0'
      } ${bgColor} ${borderColor}`}
    >
      <Icon className={`w-5 h-5 flex-shrink-0 mt-0.5 ${textColor}`} />
      <p className={`text-sm flex-1 ${textColor}`}>{toast.message}</p>
      <button
        onClick={handleDismiss}
        className={`flex-shrink-0 ${textColor} hover:opacity-70 transition-opacity`}
      >
        <X className="w-4 h-4" />
      </button>
    </div>
  )
}

/**
 * Toast container component
 * Positioned at top-right of screen
 */
export function ToastContainer({ toasts, onDismiss }: { toasts: Toast[]; onDismiss: (id: string) => void }) {
  return (
    <div className="fixed top-4 right-4 z-50 space-y-3">
      {toasts.map((toast) => (
        <Toast key={toast.id} toast={toast} onDismiss={onDismiss} />
      ))}
    </div>
  )
}

/**
 * Get toast styling based on type
 */
function getToastStyles(type: ToastType) {
  switch (type) {
    case 'success':
      return {
        icon: CheckCircle,
        bgColor: 'bg-green-50',
        textColor: 'text-green-800',
        borderColor: 'border-green-200',
      }
    case 'error':
      return {
        icon: XCircle,
        bgColor: 'bg-red-50',
        textColor: 'text-red-800',
        borderColor: 'border-red-200',
      }
    case 'info':
      return {
        icon: AlertCircle,
        bgColor: 'bg-blue-50',
        textColor: 'text-blue-800',
        borderColor: 'border-blue-200',
      }
  }
}

/**
 * Hook for managing toasts
 */
export function useToast() {
  const [toasts, setToasts] = useState<Toast[]>([])

  const showToast = (type: ToastType, message: string, duration?: number) => {
    const id = Math.random().toString(36).substring(7)
    const toast: Toast = { id, type, message, duration }
    setToasts((prev) => [...prev, toast])
  }

  const dismissToast = (id: string) => {
    setToasts((prev) => prev.filter((t) => t.id !== id))
  }

  return {
    toasts,
    showToast,
    dismissToast,
    success: (message: string, duration?: number) => showToast('success', message, duration),
    error: (message: string, duration?: number) => showToast('error', message, duration),
    info: (message: string, duration?: number) => showToast('info', message, duration),
  }
}
