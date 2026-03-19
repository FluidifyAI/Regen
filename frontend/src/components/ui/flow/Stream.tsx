import { motion } from 'framer-motion'
import { CANVAS, COLORS } from './constants'

const STREAM_PATH = 'M 0,200 C 150,195 300,190 400,200 S 600,215 800,210'
const PARTICLE_COUNT = 12

export function Stream() {
  return (
    <g>
      {/* Glow layer — wide, low opacity */}
      <path
        d={STREAM_PATH}
        fill="none"
        stroke={COLORS.coral}
        strokeWidth={18}
        strokeOpacity={0.1}
        filter="url(#watercolor)"
      />

      {/* Core stream line with gradient */}
      <path
        d={STREAM_PATH}
        fill="none"
        stroke="url(#streamGradient)"
        strokeWidth={2.5}
        strokeOpacity={0.6}
        strokeLinecap="round"
      />

      {/* Ambient particles flowing left to right */}
      {Array.from({ length: PARTICLE_COUNT }, (_, i) => (
        <motion.circle
          key={i}
          r={2.5}
          fill={COLORS.coral}
          opacity={0.5}
          cy={195 + Math.sin(i * 1.3) * 8}
          animate={{
            cx: [-10, CANVAS.width + 10],
          }}
          transition={{
            duration: 8,
            ease: 'linear',
            repeat: Infinity,
            delay: i * 0.7,
          }}
        />
      ))}
    </g>
  )
}
