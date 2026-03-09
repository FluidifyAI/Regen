/**
 * BackgroundAnimation — three ambient orbs with Framer Motion.
 * Visible, clearly drifting, but not distracting.
 */
import { motion } from 'framer-motion'

const ORBS = [
  {
    id: 1,
    style: { top: '-5%', left: '-8%', width: 480, height: 480 },
    color: '#f06292',
    opacity: 0.28,
    animate: { x: [0, 40, -20, 0], y: [0, -25, 30, 0] },
    duration: 9,
    delay: 0,
  },
  {
    id: 2,
    style: { top: '10%', right: '-8%', width: 360, height: 360 },
    color: '#b02b52',
    opacity: 0.20,
    animate: { x: [0, -30, 15, 0], y: [0, 28, -20, 0] },
    duration: 11,
    delay: 1.5,
  },
  {
    id: 3,
    style: { bottom: '-8%', left: '25%', width: 320, height: 320 },
    color: '#f06292',
    opacity: 0.18,
    animate: { x: [0, 20, -10, 0], y: [0, -18, 22, 0] },
    duration: 8,
    delay: 3,
  },
]

export function BackgroundAnimation() {
  return (
    <div
      aria-hidden="true"
      className="pointer-events-none fixed inset-0 overflow-hidden"
      style={{ zIndex: 0 }}
    >
      {ORBS.map((orb) => (
        <motion.div
          key={orb.id}
          className="absolute rounded-full"
          style={{
            ...orb.style,
            backgroundColor: orb.color,
            opacity: orb.opacity,
            filter: 'blur(60px)',
          }}
          animate={orb.animate}
          transition={{
            duration: orb.duration,
            delay: orb.delay,
            repeat: Infinity,
            repeatType: 'mirror',
            ease: 'easeInOut',
          }}
        />
      ))}
    </div>
  )
}
