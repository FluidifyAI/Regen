import { useState, useCallback } from 'react'
import { apiClient } from '../api/client'

interface VAPIDKeyResponse {
  vapid_public_key: string
}

type PushState = 'idle' | 'requesting' | 'subscribed' | 'denied' | 'error' | 'unsupported'

interface UsePushNotificationsResult {
  /** Current subscription state */
  state: PushState
  /** True when Web Push is fully supported (SW + PushManager + Notification) */
  isSupported: boolean
  /** Request notification permission and register a Web Push subscription */
  subscribe: () => Promise<void>
  /** Unregister the current Web Push subscription */
  unsubscribe: () => Promise<void>
}

/** Convert a base64url string to a Uint8Array (required by PushManager.subscribe). */
function base64urlToUint8Array(base64url: string): Uint8Array<ArrayBuffer> {
  const base64 = base64url.replace(/-/g, '+').replace(/_/g, '/')
  const padded = base64.padEnd(base64.length + ((4 - (base64.length % 4)) % 4), '=')
  const raw = atob(padded)
  return Uint8Array.from(raw, (c) => c.charCodeAt(0))
}

function isPushSupported(): boolean {
  return (
    'serviceWorker' in navigator &&
    'PushManager' in window &&
    'Notification' in window
  )
}

export function usePushNotifications(): UsePushNotificationsResult {
  const [state, setState] = useState<PushState>(() => {
    if (!isPushSupported()) return 'unsupported'
    if (Notification.permission === 'denied') return 'denied'
    return 'idle'
  })

  const subscribe = useCallback(async () => {
    if (!isPushSupported()) {
      setState('unsupported')
      return
    }

    setState('requesting')
    try {
      // 1. Fetch VAPID public key from backend.
      const { vapid_public_key } = await apiClient.get<VAPIDKeyResponse>('/api/v1/push/vapid-public-key')
      if (!vapid_public_key) {
        // Server has Web Push disabled — silently abort.
        setState('idle')
        return
      }

      // 2. Request notification permission.
      const permission = await Notification.requestPermission()
      if (permission !== 'granted') {
        setState('denied')
        return
      }

      // 3. Get the active service worker registration.
      const registration = await navigator.serviceWorker.ready

      // 4. Subscribe with PushManager.
      const subscription = await registration.pushManager.subscribe({
        userVisibleOnly: true,
        applicationServerKey: base64urlToUint8Array(vapid_public_key),
      })

      // 5. Serialise and register with backend.
      const json = subscription.toJSON()
      await apiClient.post('/api/v1/push/web/register', {
        endpoint: json.endpoint,
        p256dh: json.keys?.p256dh,
        auth: json.keys?.auth,
      })

      setState('subscribed')
    } catch (err) {
      console.error('[push] subscribe failed', err)
      setState('error')
    }
  }, [])

  const unsubscribe = useCallback(async () => {
    try {
      const registration = await navigator.serviceWorker.ready
      const subscription = await registration.pushManager.getSubscription()
      if (subscription) {
        const endpoint = subscription.endpoint
        await subscription.unsubscribe()
        await apiClient.post('/api/v1/push/web/unregister', { endpoint })
      }
      setState('idle')
    } catch (err) {
      console.error('[push] unsubscribe failed', err)
      setState('error')
    }
  }, [])

  return {
    state,
    isSupported: isPushSupported(),
    subscribe,
    unsubscribe,
  }
}
