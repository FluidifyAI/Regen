import { useState, useEffect } from 'react'
import { Download, X } from 'lucide-react'
import { useMediaQuery } from '../hooks/useMediaQuery'

interface BeforeInstallPromptEvent extends Event {
  prompt(): Promise<void>
  readonly userChoice: Promise<{ outcome: 'accepted' | 'dismissed' }>
}

/**
 * Shows an "Add to Home Screen" banner on mobile when the browser fires
 * beforeinstallprompt. Dismissed state is remembered in sessionStorage so
 * the banner doesn't reappear within the same session.
 */
export function InstallPrompt() {
  const isMobile = useMediaQuery('(max-width: 767px)')
  const [deferredPrompt, setDeferredPrompt] = useState<BeforeInstallPromptEvent | null>(null)
  const [dismissed, setDismissed] = useState<boolean>(() => {
    try {
      return sessionStorage.getItem('pwa-install-dismissed') === '1'
    } catch {
      return false
    }
  })

  useEffect(() => {
    const handler = (e: Event) => {
      e.preventDefault()
      setDeferredPrompt(e as BeforeInstallPromptEvent)
    }
    window.addEventListener('beforeinstallprompt', handler)
    return () => window.removeEventListener('beforeinstallprompt', handler)
  }, [])

  const handleInstall = async () => {
    if (!deferredPrompt) return
    await deferredPrompt.prompt()
    const { outcome } = await deferredPrompt.userChoice
    if (outcome === 'accepted') {
      setDeferredPrompt(null)
    }
  }

  const handleDismiss = () => {
    setDismissed(true)
    try {
      sessionStorage.setItem('pwa-install-dismissed', '1')
    } catch {
      // sessionStorage not available (e.g. private mode with blocked storage)
    }
  }

  // Only render on mobile, when there is a pending prompt, and not dismissed.
  if (!isMobile || !deferredPrompt || dismissed) return null

  return (
    <div
      role="banner"
      aria-label="Install Regen app"
      className="fixed bottom-0 inset-x-0 z-50 flex items-center gap-3 bg-slate-800 border-t border-slate-700 px-4 py-3 shadow-lg"
    >
      <img src="/icon-192.png" alt="" className="h-10 w-10 rounded-lg flex-shrink-0" />
      <div className="flex-1 min-w-0">
        <p className="text-sm font-semibold text-white leading-tight">Add Regen to Home Screen</p>
        <p className="text-xs text-slate-400 leading-tight">Get faster access and push alerts</p>
      </div>
      <button
        onClick={handleInstall}
        className="flex items-center gap-1.5 bg-indigo-600 hover:bg-indigo-500 active:bg-indigo-700 text-white text-sm font-medium px-3 py-1.5 rounded-lg flex-shrink-0 transition-colors"
      >
        <Download className="h-4 w-4" aria-hidden="true" />
        Install
      </button>
      <button
        onClick={handleDismiss}
        aria-label="Dismiss install prompt"
        className="p-1.5 text-slate-400 hover:text-white transition-colors flex-shrink-0"
      >
        <X className="h-4 w-4" aria-hidden="true" />
      </button>
    </div>
  )
}
