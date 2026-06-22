import { LOGO_ICON_DATA_URI } from '../../assets/logoIcon'

interface LogoProps {
  /** Size in pixels (renders as a square). Default: 32 */
  size?: number
  className?: string
}

/**
 * Fluidify Regen brand logo — inlined as a data URI so it renders in PostHog
 * session replays (played back cross-origin, where same-origin assets can't be
 * re-fetched). See src/assets/logoIcon.ts.
 */
export function Logo({ size = 32, className = '' }: LogoProps) {
  return (
    <img
      src={LOGO_ICON_DATA_URI}
      alt="Fluidify Regen"
      width={size}
      height={size}
      className={`object-contain ${className}`}
      draggable={false}
    />
  )
}
