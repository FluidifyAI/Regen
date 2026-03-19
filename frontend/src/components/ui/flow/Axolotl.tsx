import { motion } from 'framer-motion'
import { COLORS, LOOP_DURATION } from './constants'

/* ── External gills — feathery wisps that sway ──────────────── */

const GILLS: Array<{ x1: number; y1: number; x2: number; y2: number; delay: number }> = [
  { x1: 6, y1: -5, x2: 2, y2: -12, delay: 0 },
  { x1: 8, y1: -6, x2: 6, y2: -14, delay: 0.15 },
  { x1: 10, y1: -7, x2: 12, y2: -14, delay: 0.3 },
]

function Gills() {
  return (
    <g>
      {GILLS.map((g, i) => (
        <motion.line
          key={i}
          x1={g.x1}
          y1={g.y1}
          stroke={COLORS.coralLight}
          strokeWidth={1}
          strokeOpacity={0.6}
          strokeLinecap="round"
          animate={{
            x2: [g.x2 - 2, g.x2 + 2, g.x2 - 2],
            y2: [g.y2, g.y2 - 1, g.y2],
          }}
          transition={{
            duration: 0.8,
            ease: 'easeInOut',
            repeat: Infinity,
            repeatType: 'reverse',
            delay: g.delay,
          }}
        />
      ))}
    </g>
  )
}

/* ── Tail — wagging path ────────────────────────────────────── */

function Tail() {
  return (
    <motion.path
      fill="none"
      stroke={COLORS.coral}
      strokeWidth={1.5}
      strokeOpacity={0.7}
      strokeLinecap="round"
      animate={{
        d: [
          'M -10,0 Q -16,-4 -14,2',
          'M -10,0 Q -16,4 -14,-2',
          'M -10,0 Q -16,-4 -14,2',
        ],
      }}
      transition={{
        duration: 1,
        ease: 'easeInOut',
        repeat: Infinity,
      }}
    />
  )
}

/* ── Main component ─────────────────────────────────────────── */

export function Axolotl() {
  return (
    <motion.g
      animate={{
        x: [40, 150, 310, 500, 700, 700, 40],
        y: [240, 245, 235, 240, 245, 245, 240],
      }}
      transition={{
        duration: LOOP_DURATION,
        ease: 'easeInOut',
        repeat: Infinity,
        times: [0, 0.15, 0.4, 0.65, 0.88, 0.95, 1],
      }}
    >
      {/* Tail */}
      <Tail />

      {/* Body */}
      <ellipse cx={0} cy={0} rx={10} ry={5} fill={COLORS.coral} fillOpacity={0.7} />

      {/* Head */}
      <circle cx={10} cy={-1} r={5} fill={COLORS.coral} fillOpacity={0.8} />

      {/* Eye — white sclera */}
      <circle cx={12} cy={-3} r={1.2} fill={COLORS.white} />
      {/* Eye — dark pupil */}
      <circle cx={12.3} cy={-3} r={0.7} fill="#1a1a1a" />

      {/* Smile */}
      <path
        d="M 10,0 Q 12,1.5 14,0"
        fill="none"
        stroke={COLORS.coralDeep}
        strokeWidth={0.6}
        strokeOpacity={0.5}
        strokeLinecap="round"
      />

      {/* External gills */}
      <Gills />

      {/* Legs */}
      <line x1={-4} y1={4} x2={-6} y2={8} stroke={COLORS.coral} strokeWidth={0.8} strokeOpacity={0.5} />
      <line x1={4} y1={4} x2={6} y2={8} stroke={COLORS.coral} strokeWidth={0.8} strokeOpacity={0.5} />
    </motion.g>
  )
}
