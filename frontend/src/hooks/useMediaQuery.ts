import { useState, useEffect } from 'react'

/**
 * Returns true when the given CSS media query matches.
 * Updates reactively as the viewport changes.
 *
 * Example:
 *   const isMobile = useMediaQuery('(max-width: 639px)')
 */
export function useMediaQuery(query: string): boolean {
  const getMatches = () => {
    if (typeof window === 'undefined') return false
    return window.matchMedia(query).matches
  }

  const [matches, setMatches] = useState<boolean>(getMatches)

  useEffect(() => {
    const mql = window.matchMedia(query)
    setMatches(mql.matches)

    const onChange = (e: MediaQueryListEvent) => setMatches(e.matches)
    // addEventListener is preferred over the deprecated addListener.
    mql.addEventListener('change', onChange)
    return () => mql.removeEventListener('change', onChange)
  }, [query])

  return matches
}
