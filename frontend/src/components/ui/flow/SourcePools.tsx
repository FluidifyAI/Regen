import { motion } from 'framer-motion'
import { COLORS, LOOP_DURATION } from './constants'

/* ── Source pool definitions ─────────────────────────────────── */

interface SourcePool {
  cx: number
  cy: number
  color: string
  icon: string
  active: true
}

interface StubPool {
  cx: number
  cy: number
  icon: string
  active: false
}

const ACTIVE_POOLS: SourcePool[] = [
  { cx: 60, cy: 160, color: COLORS.amber, icon: 'prometheus', active: true },
  { cx: 110, cy: 185, color: '#F46800', icon: 'grafana', active: true },
  { cx: 50, cy: 220, color: COLORS.blue, icon: 'amazoncloudwatch', active: true },
]

const STUB_POOLS: StubPool[] = [
  { cx: 130, cy: 145, icon: 'datadog', active: false },
  { cx: 140, cy: 230, icon: 'newrelic', active: false },
]

/* ── Helpers ─────────────────────────────────────────────────── */

function diamondPath(cx: number, cy: number) {
  return `M ${cx},${cy - 15} L ${cx + 20},${cy} L ${cx},${cy + 10} L ${cx - 20},${cy} Z`
}

function iconUrl(slug: string) {
  return `https://cdn.jsdelivr.net/npm/simple-icons@v11/icons/${slug}.svg`
}

/* ── Sub-components ──────────────────────────────────────────── */

function ActiveBasin({ cx, cy, color, icon }: SourcePool) {
  return (
    <g>
      {/* Pulsing fill */}
      <motion.path
        d={diamondPath(cx, cy)}
        fill={color}
        stroke={color}
        strokeWidth={1.2}
        strokeOpacity={0.5}
        animate={{ fillOpacity: [0.15, 0.25, 0.15] }}
        transition={{ duration: 3, ease: 'easeInOut', repeat: Infinity }}
      />

      {/* Brand icon */}
      <image
        href={iconUrl(icon)}
        x={cx - 8}
        y={cy - 10}
        width={16}
        height={16}
        opacity={0.7}
      />

      {/* Ambient bubbles */}
      {[0, 1, 2].map((i) => (
        <motion.circle
          key={i}
          cx={cx - 6 + i * 6}
          r={1.5}
          fill={color}
          animate={{
            cy: [cy - 2, cy - 18],
            opacity: [0.5, 0],
          }}
          transition={{
            duration: 2,
            ease: 'easeOut',
            repeat: Infinity,
            delay: i * 0.7,
          }}
        />
      ))}
    </g>
  )
}

function StubBasin({ cx, cy, icon }: StubPool) {
  return (
    <g opacity={0.25}>
      <path
        d={diamondPath(cx, cy)}
        fill="none"
        stroke={COLORS.gray}
        strokeWidth={1}
        strokeDasharray="3 2"
      />
      <image
        href={iconUrl(icon)}
        x={cx - 6}
        y={cy - 8}
        width={12}
        height={12}
        opacity={0.5}
      />
    </g>
  )
}

/* ── Alert droplet ───────────────────────────────────────────── */

const DROPLET_PATH_X = [60, 80, 120, 170]
const DROPLET_PATH_Y = [160, 170, 190, 200]
const DROPLET_TRAVEL = 2.5
const DROPLET_DELAY = 0.5
const DROPLET_REPEAT_DELAY = LOOP_DURATION - DROPLET_TRAVEL

function AlertDroplet() {
  return (
    <g>
      {/* Outer glow */}
      <motion.circle
        r={8}
        fill={COLORS.red}
        opacity={0}
        filter="url(#glow)"
        animate={{
          cx: DROPLET_PATH_X,
          cy: DROPLET_PATH_Y,
          opacity: [0, 0.2, 0.2, 0],
        }}
        transition={{
          duration: DROPLET_TRAVEL,
          ease: 'easeInOut',
          repeat: Infinity,
          delay: DROPLET_DELAY,
          repeatDelay: DROPLET_REPEAT_DELAY,
        }}
      />

      {/* Core */}
      <motion.circle
        r={4}
        fill={COLORS.red}
        opacity={0}
        animate={{
          cx: DROPLET_PATH_X,
          cy: DROPLET_PATH_Y,
          opacity: [0, 0.9, 0.9, 0],
        }}
        transition={{
          duration: DROPLET_TRAVEL,
          ease: 'easeInOut',
          repeat: Infinity,
          delay: DROPLET_DELAY,
          repeatDelay: DROPLET_REPEAT_DELAY,
        }}
      />

      {/* Ring */}
      <motion.circle
        r={6}
        fill="none"
        stroke={COLORS.red}
        strokeWidth={1}
        opacity={0}
        animate={{
          cx: DROPLET_PATH_X,
          cy: DROPLET_PATH_Y,
          opacity: [0, 0.6, 0.6, 0],
        }}
        transition={{
          duration: DROPLET_TRAVEL,
          ease: 'easeInOut',
          repeat: Infinity,
          delay: DROPLET_DELAY,
          repeatDelay: DROPLET_REPEAT_DELAY,
        }}
      />

      {/* "critical" label */}
      <motion.text
        fontSize={7}
        fill={COLORS.red}
        fontWeight={600}
        textAnchor="middle"
        opacity={0}
        animate={{
          x: DROPLET_PATH_X,
          y: DROPLET_PATH_Y.map((v) => v - 12),
          opacity: [0, 0.8, 0.8, 0],
        }}
        transition={{
          duration: DROPLET_TRAVEL,
          ease: 'easeInOut',
          repeat: Infinity,
          delay: DROPLET_DELAY,
          repeatDelay: DROPLET_REPEAT_DELAY,
        }}
      >
        critical
      </motion.text>
    </g>
  )
}

/* ── Main component ──────────────────────────────────────────── */

export function SourcePools() {
  return (
    <g>
      {/* Active source pools */}
      {ACTIVE_POOLS.map((pool) => (
        <ActiveBasin key={pool.icon} {...pool} />
      ))}

      {/* Coming-soon stubs */}
      {STUB_POOLS.map((pool) => (
        <StubBasin key={pool.icon} {...pool} />
      ))}

      {/* Shared "more sources coming" label */}
      <text
        x={135}
        y={252}
        fontSize={6}
        fill={COLORS.textMuted}
        opacity={0.4}
        textAnchor="middle"
      >
        more sources coming
      </text>

      {/* Alert droplet animation */}
      <AlertDroplet />
    </g>
  )
}
