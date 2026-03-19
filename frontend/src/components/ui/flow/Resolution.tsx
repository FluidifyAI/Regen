import { motion } from 'framer-motion'
import { COLORS, LOOP_DURATION } from './constants'

/* ── Hexagon helper ─────────────────────────────────────────── */

function hexPoints(cx: number, cy: number, r: number): string {
  return Array.from({ length: 6 }, (_, i) => {
    const angle = (Math.PI / 3) * i - Math.PI / 2
    return `${cx + r * Math.cos(angle)},${cy + r * Math.sin(angle)}`
  }).join(' ')
}

/* ── Calm lagoon background ─────────────────────────────────── */

function Lagoon() {
  return (
    <ellipse
      cx={730}
      cy={210}
      rx={50}
      ry={20}
      fill={COLORS.coral}
      fillOpacity={0.05}
    />
  )
}

/* ── Resolved hexagon — appears near end of loop ────────────── */

function ResolvedHexagon() {
  return (
    <motion.g
      animate={{ opacity: [0, 0, 0, 1, 0.6, 0.3] }}
      transition={{
        duration: LOOP_DURATION,
        ease: 'linear',
        repeat: Infinity,
        times: [0, 0.8, 0.9, 0.92, 0.96, 1],
      }}
    >
      <polygon
        points={hexPoints(730, 210, 14)}
        fill={COLORS.green}
        fillOpacity={0.3}
        stroke={COLORS.green}
        strokeWidth={1}
      />
      <text
        x={730}
        y={213}
        fontSize={5}
        fontFamily="monospace"
        fontWeight={600}
        fill={COLORS.green}
        textAnchor="middle"
      >
        RESOLVED
      </text>
    </motion.g>
  )
}

/* ── Historical reef — past resolved incidents ──────────────── */

const REEF: Array<{ cx: number; cy: number; r: number }> = [
  { cx: 704, cy: 232, r: 6 },
  { cx: 726, cy: 238, r: 5 },
  { cx: 744, cy: 230, r: 5.5 },
]

function HistoricalReef() {
  return (
    <g>
      {REEF.map((h, i) => (
        <polygon
          key={i}
          points={hexPoints(h.cx, h.cy, h.r)}
          fill={COLORS.grayLight}
          fillOpacity={0.2}
          stroke="none"
        />
      ))}
    </g>
  )
}

/* ── Main component ─────────────────────────────────────────── */

export function Resolution() {
  return (
    <g>
      <Lagoon />
      <HistoricalReef />
      <ResolvedHexagon />
    </g>
  )
}
