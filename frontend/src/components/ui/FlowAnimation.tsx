import { useRef } from 'react'
import { useInView } from 'framer-motion'
import { CANVAS, COLORS } from './flow/constants'
import { Stream } from './flow/Stream'
import { SourcePools } from './flow/SourcePools'
import { Nexus } from './flow/Nexus'

export function FlowAnimation() {
  const ref = useRef<HTMLDivElement>(null)
  const isInView = useInView(ref, { amount: 0.3, once: false })

  return (
    <div ref={ref} className="w-full max-w-4xl mx-auto px-4">
      <svg
        viewBox={`0 0 ${CANVAS.width} ${CANVAS.height}`}
        className="w-full h-auto"
        role="img"
        aria-label="Animated flow diagram showing how alerts are processed into incidents by Fluidify Regen"
      >
        <defs>
          <linearGradient id="streamGradient" x1="0%" y1="0%" x2="100%" y2="0%">
            <stop offset="0%" stopColor={COLORS.coral} />
            <stop offset="50%" stopColor={COLORS.coralDeep} />
            <stop offset="100%" stopColor={COLORS.purple} />
          </linearGradient>

          <filter id="watercolor">
            <feGaussianBlur stdDeviation="3" />
          </filter>

          <filter id="glow">
            <feGaussianBlur stdDeviation="4" result="blur" />
            <feMerge>
              <feMergeNode in="blur" />
              <feMergeNode in="SourceGraphic" />
            </feMerge>
          </filter>
        </defs>

        {isInView && (
          <>
            <Stream />
            <SourcePools />
            <Nexus />
          </>
        )}
      </svg>
    </div>
  )
}
