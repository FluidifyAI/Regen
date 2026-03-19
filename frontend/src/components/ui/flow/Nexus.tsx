import { motion } from 'framer-motion'
import { COLORS, LOOP_DURATION } from './constants'

/* ── Helpers ─────────────────────────────────────────────────── */

const CX = 310
const CY = 200

function iconUrl(slug: string) {
  return `https://cdn.jsdelivr.net/npm/simple-icons@v11/icons/${slug}.svg`
}

/* ── Whirlpool ───────────────────────────────────────────────── */

function Whirlpool() {
  const origin = { transformOrigin: `${CX}px ${CY}px` }

  return (
    <g>
      {/* Outer ring */}
      <motion.ellipse
        cx={CX}
        cy={CY}
        rx={60}
        ry={30}
        fill="none"
        stroke={COLORS.coral}
        strokeWidth={1}
        strokeOpacity={0.4}
        style={origin}
        animate={{ rotate: 360 }}
        transition={{ duration: 10, ease: 'linear', repeat: Infinity }}
      />

      {/* Middle ring */}
      <motion.ellipse
        cx={CX}
        cy={CY}
        rx={40}
        ry={20}
        fill="none"
        stroke={COLORS.coralDeep}
        strokeWidth={1}
        strokeOpacity={0.5}
        style={origin}
        animate={{ rotate: -360 }}
        transition={{ duration: 7, ease: 'linear', repeat: Infinity }}
      />

      {/* Inner ring (filled) */}
      <motion.ellipse
        cx={CX}
        cy={CY}
        rx={20}
        ry={10}
        fill={COLORS.coral}
        fillOpacity={0.1}
        stroke="none"
        style={origin}
        animate={{ rotate: 360 }}
        transition={{ duration: 5, ease: 'linear', repeat: Infinity }}
      />

      {/* Center glow */}
      <circle
        cx={CX}
        cy={CY}
        r={8}
        fill={COLORS.coral}
        opacity={0.15}
        filter="url(#glow)"
      />
    </g>
  )
}

/* ── Incident hexagon ────────────────────────────────────────── */

const HEX_DURATION = 7
const HEX_DELAY = 2.5
const HEX_REPEAT_DELAY = LOOP_DURATION - HEX_DURATION - HEX_DELAY

const HEX_X_PATH = [170, 310, 310, 430]
const HEX_Y_PATH = [200, 200, 200, 200]

function IncidentHexagon() {
  return (
    <g>
      {/* Hexagon shape */}
      <motion.polygon
        points="0,-10 8.7,-5 8.7,5 0,10 -8.7,5 -8.7,-5"
        fill={COLORS.coral}
        fillOpacity={0.3}
        stroke={COLORS.coralDeep}
        strokeWidth={1}
        animate={{
          x: HEX_X_PATH,
          y: HEX_Y_PATH,
          scale: [0, 1, 1, 0],
          opacity: [0, 1, 1, 0],
        }}
        transition={{
          duration: HEX_DURATION,
          ease: 'easeInOut',
          repeat: Infinity,
          delay: HEX_DELAY,
          repeatDelay: HEX_REPEAT_DELAY,
          times: [0, 0.25, 0.75, 1],
        }}
      />

      {/* INC-042 label */}
      <motion.text
        fontSize={6}
        fontFamily="monospace"
        fill={COLORS.coralDeep}
        textAnchor="middle"
        fontWeight={600}
        animate={{
          x: HEX_X_PATH,
          y: HEX_Y_PATH.map((v) => v - 16),
          opacity: [0, 0.9, 0.9, 0],
        }}
        transition={{
          duration: HEX_DURATION,
          ease: 'easeInOut',
          repeat: Infinity,
          delay: HEX_DELAY,
          repeatDelay: HEX_REPEAT_DELAY,
          times: [0, 0.25, 0.75, 1],
        }}
      >
        INC-042
      </motion.text>
    </g>
  )
}

/* ── Channel tendrils ────────────────────────────────────────── */

interface Tendril {
  endX: number
  endY: number
  color: string
  icon: string
  delay: number
  label?: string
  stub?: boolean
}

const TENDRILS: Tendril[] = [
  {
    endX: 260,
    endY: 130,
    color: '#4A154B',
    icon: 'slack',
    delay: 3.8,
    label: '#inc-042',
  },
  {
    endX: 360,
    endY: 125,
    color: '#6264A7',
    icon: 'microsoftteams',
    delay: 4.3,
  },
  {
    endX: 310,
    endY: 120,
    color: '#26A5E4',
    icon: 'telegram',
    delay: 4.5,
  },
  {
    endX: 370,
    endY: 140,
    color: COLORS.gray,
    icon: 'discord',
    delay: 4.7,
    stub: true,
  },
]

const TENDRIL_DRAW = 1.5
const tendrilRepeatDelay = (delay: number) => LOOP_DURATION - TENDRIL_DRAW - delay

function tendrilPath(endX: number, endY: number) {
  const midX = (CX + endX) / 2
  const midY = CY - 30
  return `M ${CX},${CY} Q ${midX},${midY} ${endX},${endY}`
}

function ChannelTendrils() {
  return (
    <g>
      {TENDRILS.map((t) => {
        const d = tendrilPath(t.endX, t.endY)
        const isStub = t.stub === true

        return (
          <g key={t.icon}>
            {/* Path line */}
            <motion.path
              d={d}
              fill="none"
              stroke={t.color}
              strokeWidth={isStub ? 1 : 1.5}
              strokeDasharray={isStub ? '3 2' : undefined}
              opacity={isStub ? 0.2 : 1}
              initial={{ pathLength: 0, opacity: 0 }}
              animate={{
                pathLength: [0, 1, 1, 0],
                opacity: isStub ? [0, 0.2, 0.2, 0] : [0, 0.7, 0.7, 0],
              }}
              transition={{
                duration: TENDRIL_DRAW,
                ease: 'easeOut',
                repeat: Infinity,
                delay: t.delay,
                repeatDelay: tendrilRepeatDelay(t.delay),
                times: [0, 0.4, 0.8, 1],
              }}
            />

            {/* Icon at tip */}
            <motion.image
              href={iconUrl(t.icon)}
              x={t.endX - 7}
              y={t.endY - 7}
              width={14}
              height={14}
              animate={{
                opacity: isStub ? [0, 0.2, 0.2, 0] : [0, 0.8, 0.8, 0],
              }}
              transition={{
                duration: TENDRIL_DRAW,
                ease: 'easeOut',
                repeat: Infinity,
                delay: t.delay,
                repeatDelay: tendrilRepeatDelay(t.delay),
                times: [0, 0.4, 0.8, 1],
              }}
            />

            {/* Channel label (Slack only) */}
            {t.label && (
              <motion.text
                x={t.endX}
                y={t.endY - 12}
                fontSize={6}
                fontFamily="monospace"
                fill={t.color}
                textAnchor="middle"
                fontWeight={500}
                animate={{ opacity: [0, 0.8, 0.8, 0] }}
                transition={{
                  duration: TENDRIL_DRAW,
                  ease: 'easeOut',
                  repeat: Infinity,
                  delay: t.delay,
                  repeatDelay: tendrilRepeatDelay(t.delay),
                  times: [0, 0.4, 0.8, 1],
                }}
              >
                {t.label}
              </motion.text>
            )}
          </g>
        )
      })}
    </g>
  )
}

/* ── On-Call ACK ─────────────────────────────────────────────── */

const ACK_DELAY = 5
const ACK_VISIBLE = 2
const ACK_REPEAT_DELAY = LOOP_DURATION - ACK_VISIBLE - ACK_DELAY

function OnCallAck() {
  return (
    <g>
      {/* Avatar circle */}
      <motion.circle
        cx={380}
        cy={175}
        r={10}
        fill={COLORS.coral}
        fillOpacity={0.2}
        stroke={COLORS.coral}
        strokeWidth={1}
        animate={{ opacity: [0, 0.8, 0.8, 0] }}
        transition={{
          duration: ACK_VISIBLE,
          ease: 'easeInOut',
          repeat: Infinity,
          delay: ACK_DELAY,
          repeatDelay: ACK_REPEAT_DELAY,
          times: [0, 0.15, 0.85, 1],
        }}
      />

      {/* Headset emoji */}
      <motion.text
        x={380}
        y={179}
        fontSize={10}
        textAnchor="middle"
        animate={{ opacity: [0, 1, 1, 0] }}
        transition={{
          duration: ACK_VISIBLE,
          ease: 'easeInOut',
          repeat: Infinity,
          delay: ACK_DELAY,
          repeatDelay: ACK_REPEAT_DELAY,
          times: [0, 0.15, 0.85, 1],
        }}
      >
        🎧
      </motion.text>

      {/* ACK label */}
      <motion.text
        x={380}
        y={160}
        fontSize={7}
        fontFamily="monospace"
        fontWeight={700}
        fill={COLORS.amber}
        textAnchor="middle"
        animate={{ opacity: [0, 1, 1, 0] }}
        transition={{
          duration: ACK_VISIBLE,
          ease: 'easeInOut',
          repeat: Infinity,
          delay: ACK_DELAY,
          repeatDelay: ACK_REPEAT_DELAY,
          times: [0, 0.15, 0.85, 1],
        }}
      >
        ACK
      </motion.text>
    </g>
  )
}

/* ── Timeline rings ──────────────────────────────────────────── */

interface TimelineRing {
  label: string
  color: string
  delay: number
}

const TIMELINE_RINGS: TimelineRing[] = [
  { label: 'alert linked', color: COLORS.red, delay: 5.6 },
  { label: 'channel created', color: COLORS.coral, delay: 5.8 },
  { label: 'acknowledged', color: COLORS.amber, delay: 6.0 },
  { label: 'escalated', color: COLORS.coralDeep, delay: 6.2 },
]

const RING_DURATION = 1.5

function TimelineRings() {
  return (
    <g>
      {TIMELINE_RINGS.map((ring, i) => {
        const cy = 245 + i * 10
        const repeatDelay = LOOP_DURATION - RING_DURATION - ring.delay

        return (
          <g key={ring.label}>
            {/* Ellipse ring */}
            <motion.ellipse
              cx={CX}
              cy={cy}
              rx={18}
              ry={4}
              fill={ring.color}
              fillOpacity={0.15}
              stroke={ring.color}
              strokeWidth={0.8}
              strokeOpacity={0.5}
              animate={{
                scale: [0, 1, 1, 0],
                opacity: [0, 0.8, 0.8, 0],
              }}
              transition={{
                duration: RING_DURATION,
                ease: 'easeOut',
                repeat: Infinity,
                delay: ring.delay,
                repeatDelay,
                times: [0, 0.2, 0.8, 1],
              }}
              style={{ transformOrigin: `${CX}px ${cy}px` }}
            />

            {/* Label */}
            <motion.text
              x={CX + 24}
              y={cy + 3}
              fontSize={5.5}
              fontFamily="monospace"
              fill={ring.color}
              animate={{ opacity: [0, 0.8, 0.8, 0] }}
              transition={{
                duration: RING_DURATION,
                ease: 'easeOut',
                repeat: Infinity,
                delay: ring.delay,
                repeatDelay,
                times: [0, 0.2, 0.8, 1],
              }}
            >
              {ring.label}
            </motion.text>
          </g>
        )
      })}
    </g>
  )
}

/* ── Main component ──────────────────────────────────────────── */

export function Nexus() {
  return (
    <g>
      <Whirlpool />
      <IncidentHexagon />
      <ChannelTendrils />
      <OnCallAck />
      <TimelineRings />
    </g>
  )
}
