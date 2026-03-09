interface LogoProps {
  /** Size in pixels (renders as a square). Default: 32 */
  size?: number
  className?: string
}

/**
 * Fluidify Regen brand logo — uses logo.png from /public.
 */
export function Logo({ size = 32, className = '' }: LogoProps) {
  return (
    <img
      src="/logo.png"
      alt="Fluidify Regen"
      width={size}
      height={size}
      className={`object-contain ${className}`}
      draggable={false}
    />
  )
}
