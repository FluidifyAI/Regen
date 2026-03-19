import { motion } from 'framer-motion'
import { COLORS, LOOP_DURATION } from './constants'

/* ── Jellyfish — Summarization Agent ──────────────────────────── */

const JELLY_DELAY = 7
const JELLY_DURATION = 3
const JELLY_REPEAT_DELAY = LOOP_DURATION - JELLY_DURATION - JELLY_DELAY

function Jellyfish() {
  const cx = 510
  const tentacleOffsets = [-6, -2, 2, 6]

  return (
    <motion.g
      animate={{
        y: [280, 190, 190, 280],
        opacity: [0, 1, 1, 0],
      }}
      transition={{
        duration: JELLY_DURATION,
        ease: 'easeInOut',
        repeat: Infinity,
        delay: JELLY_DELAY,
        repeatDelay: JELLY_REPEAT_DELAY,
        times: [0, 0.2, 0.8, 1],
      }}
    >
      {/* Bell */}
      <motion.ellipse
        cx={cx}
        cy={0}
        rx={14}
        fill={COLORS.coral}
        fillOpacity={0.2}
        stroke={COLORS.coral}
        strokeWidth={1}
        animate={{ ry: [10, 12, 10] }}
        transition={{
          duration: 1.2,
          ease: 'easeInOut',
          repeat: Infinity,
        }}
      />

      {/* Inner sparkles */}
      <circle cx={cx - 4} cy={-2} r={1.2} fill={COLORS.coral} opacity={0.5} />
      <circle cx={cx + 3} cy={-3} r={1} fill={COLORS.coralLight} opacity={0.4} />
      <circle cx={cx} cy={2} r={0.8} fill={COLORS.coral} opacity={0.6} />

      {/* Tentacles */}
      {tentacleOffsets.map((dx, i) => (
        <motion.line
          key={i}
          x1={cx + dx}
          y1={8}
          y2={22}
          stroke={COLORS.coral}
          strokeWidth={0.8}
          strokeOpacity={0.5}
          animate={{ x2: [cx + dx - 2, cx + dx + 2, cx + dx - 2] }}
          transition={{
            duration: 1.5,
            ease: 'easeInOut',
            repeat: Infinity,
            delay: i * 0.15,
          }}
        />
      ))}
    </motion.g>
  )
}

/* ── Summary Document ─────────────────────────────────────────── */

const DOC_DELAY = 7.4
const DOC_DURATION = 2.6
const DOC_REPEAT_DELAY = LOOP_DURATION - DOC_DURATION - DOC_DELAY

function SummaryDocument() {
  const x = 530
  const y = 175

  return (
    <motion.g
      animate={{ opacity: [0, 1, 1, 0] }}
      transition={{
        duration: DOC_DURATION,
        ease: 'easeInOut',
        repeat: Infinity,
        delay: DOC_DELAY,
        repeatDelay: DOC_REPEAT_DELAY,
        times: [0, 0.15, 0.8, 1],
      }}
    >
      <rect
        x={x}
        y={y}
        width={40}
        height={24}
        rx={3}
        fill={COLORS.white}
        stroke={COLORS.coral}
        strokeWidth={0.8}
      />
      <text
        x={x + 4}
        y={y + 8}
        fontSize={5}
        fontFamily="monospace"
        fontWeight={600}
        fill={COLORS.coral}
      >
        Summary
      </text>
      <line x1={x + 4} y1={y + 12} x2={x + 32} y2={y + 12} stroke={COLORS.grayLight} strokeWidth={0.6} />
      <line x1={x + 4} y1={y + 16} x2={x + 28} y2={y + 16} stroke={COLORS.grayLight} strokeWidth={0.6} />
      <line x1={x + 4} y1={y + 20} x2={x + 24} y2={y + 20} stroke={COLORS.grayLight} strokeWidth={0.6} />
    </motion.g>
  )
}

/* ── Manta Ray — Post-Mortem Agent ────────────────────────────── */

const MANTA_DELAY = 8.5
const MANTA_DURATION = 3
const MANTA_REPEAT_DELAY = LOOP_DURATION - MANTA_DURATION - MANTA_DELAY

function MantaRay() {
  const cx = 600

  return (
    <motion.g
      animate={{
        y: [280, 195, 195, 280],
        opacity: [0, 1, 1, 0],
      }}
      transition={{
        duration: MANTA_DURATION,
        ease: 'easeInOut',
        repeat: Infinity,
        delay: MANTA_DELAY,
        repeatDelay: MANTA_REPEAT_DELAY,
        times: [0, 0.2, 0.8, 1],
      }}
    >
      {/* Body — manta ray shape */}
      <motion.path
        d={`M ${cx},0 Q ${cx - 20},-6 ${cx - 28},-2 Q ${cx - 18},2 ${cx},4 Q ${cx + 18},2 ${cx + 28},-2 Q ${cx + 20},-6 ${cx},0 Z`}
        fill={COLORS.purple}
        fillOpacity={0.25}
        stroke={COLORS.purple}
        strokeWidth={0.8}
        animate={{
          d: [
            `M ${cx},0 Q ${cx - 20},-6 ${cx - 28},-2 Q ${cx - 18},2 ${cx},4 Q ${cx + 18},2 ${cx + 28},-2 Q ${cx + 20},-6 ${cx},0 Z`,
            `M ${cx},0 Q ${cx - 20},-8 ${cx - 28},-6 Q ${cx - 18},0 ${cx},4 Q ${cx + 18},0 ${cx + 28},-6 Q ${cx + 20},-8 ${cx},0 Z`,
            `M ${cx},0 Q ${cx - 20},-6 ${cx - 28},-2 Q ${cx - 18},2 ${cx},4 Q ${cx + 18},2 ${cx + 28},-2 Q ${cx + 20},-6 ${cx},0 Z`,
          ],
        }}
        transition={{
          duration: 1.8,
          ease: 'easeInOut',
          repeat: Infinity,
        }}
      />

      {/* Tail */}
      <line
        x1={cx}
        y1={4}
        x2={cx}
        y2={16}
        stroke={COLORS.purple}
        strokeWidth={0.7}
        strokeOpacity={0.5}
      />
    </motion.g>
  )
}

/* ── Post-Mortem Document ─────────────────────────────────────── */

const PM_DOC_DELAY = 8.9
const PM_DOC_DURATION = 2.6
const PM_DOC_REPEAT_DELAY = LOOP_DURATION - PM_DOC_DURATION - PM_DOC_DELAY

function PostMortemDocument() {
  const x = 624
  const y = 175

  return (
    <motion.g
      animate={{ opacity: [0, 1, 1, 0] }}
      transition={{
        duration: PM_DOC_DURATION,
        ease: 'easeInOut',
        repeat: Infinity,
        delay: PM_DOC_DELAY,
        repeatDelay: PM_DOC_REPEAT_DELAY,
        times: [0, 0.15, 0.8, 1],
      }}
    >
      <rect
        x={x}
        y={y}
        width={48}
        height={28}
        rx={3}
        fill={COLORS.white}
        stroke={COLORS.purple}
        strokeWidth={0.8}
      />
      <text
        x={x + 4}
        y={y + 7}
        fontSize={4.5}
        fontFamily="monospace"
        fontWeight={600}
        fill={COLORS.purple}
      >
        Post-Mortem
      </text>
      <text x={x + 4} y={y + 13} fontSize={3.5} fontFamily="monospace" fill={COLORS.textMuted}>
        Timeline
      </text>
      <text x={x + 4} y={y + 18} fontSize={3.5} fontFamily="monospace" fill={COLORS.textMuted}>
        Contributing Factors
      </text>
      <text x={x + 4} y={y + 23} fontSize={3.5} fontFamily="monospace" fill={COLORS.textMuted}>
        Action Items
      </text>
    </motion.g>
  )
}

/* ── Coming-Soon Silhouettes ──────────────────────────────────── */

const SOON_DELAY = 10
const SOON_DURATION = 1.5
const SOON_REPEAT_DELAY = LOOP_DURATION - SOON_DURATION - SOON_DELAY

function ComingSoonAgents() {
  const tentacleOffsets = [-5, -2.5, 0, 2.5, 5]

  return (
    <motion.g
      opacity={0.15}
      animate={{ opacity: [0, 0.15, 0.15, 0] }}
      transition={{
        duration: SOON_DURATION,
        ease: 'easeInOut',
        repeat: Infinity,
        delay: SOON_DELAY,
        repeatDelay: SOON_REPEAT_DELAY,
        times: [0, 0.2, 0.8, 1],
      }}
    >
      {/* Triage — barracuda */}
      <motion.ellipse
        cy={270}
        rx={12}
        ry={3}
        fill={COLORS.gray}
        animate={{ cx: [490, 510, 490] }}
        transition={{
          duration: 2,
          ease: 'easeInOut',
          repeat: Infinity,
        }}
      />

      {/* Root cause — anglerfish */}
      <ellipse cx={560} cy={280} rx={8} ry={6} fill={COLORS.gray} />
      <motion.circle
        cx={568}
        cy={275}
        r={2}
        fill={COLORS.coral}
        animate={{ opacity: [0.3, 0.9, 0.3] }}
        transition={{
          duration: 1,
          ease: 'easeInOut',
          repeat: Infinity,
        }}
      />

      {/* Runbook — octopus */}
      <ellipse cx={630} cy={275} rx={7} ry={5} fill={COLORS.gray} />
      {tentacleOffsets.map((dx, i) => (
        <motion.line
          key={i}
          x1={630 + dx}
          y1={280}
          y2={290}
          stroke={COLORS.gray}
          strokeWidth={0.7}
          animate={{ x2: [630 + dx - 1.5, 630 + dx + 1.5, 630 + dx - 1.5] }}
          transition={{
            duration: 1.4,
            ease: 'easeInOut',
            repeat: Infinity,
            delay: i * 0.1,
          }}
        />
      ))}

      {/* Shared label */}
      <text
        x={560}
        y={300}
        fontSize={6}
        fontFamily="monospace"
        fill={COLORS.gray}
        textAnchor="middle"
        opacity={0.5}
      >
        coming soon
      </text>
    </motion.g>
  )
}

/* ── Main component ──────────────────────────────────────────── */

export function AIAgents() {
  return (
    <g>
      <Jellyfish />
      <SummaryDocument />
      <MantaRay />
      <PostMortemDocument />
      <ComingSoonAgents />
    </g>
  )
}
