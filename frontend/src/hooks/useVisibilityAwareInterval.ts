import { useEffect, useRef } from 'react'

/**
 * Drop-in replacement for setInterval that pauses when the browser tab is
 * hidden and fires the callback immediately when the tab becomes visible again.
 *
 * This prevents the cascade of ERR_NAME_NOT_RESOLVED / ERR_NETWORK_IO_SUSPENDED
 * errors that occur when Chrome suspends background-tab network I/O and then
 * wakes up with stale DNS and a queue of pending interval ticks.
 */
export function useVisibilityAwareInterval(
  callback: () => void,
  intervalMs: number,
  enabled = true,
): void {
  const callbackRef = useRef(callback)
  callbackRef.current = callback

  useEffect(() => {
    if (!enabled) return

    const tick = () => callbackRef.current()

    const timer = setInterval(() => {
      if (document.hidden) return
      tick()
    }, intervalMs)

    const onVisibilityChange = () => {
      if (!document.hidden) tick()
    }

    document.addEventListener('visibilitychange', onVisibilityChange)

    return () => {
      clearInterval(timer)
      document.removeEventListener('visibilitychange', onVisibilityChange)
    }
  }, [intervalMs, enabled])
}
