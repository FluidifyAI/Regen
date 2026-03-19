import { COLORS } from './constants'

export function ContrastStrip() {
  return (
    <g transform="translate(0, 360)">
      {/* Divider line */}
      <line x1={0} y1={0} x2={800} y2={0} stroke={COLORS.grayLight} strokeOpacity={0.3} />

      {/* Left half — stagnant/expensive */}
      <rect x={0} y={5} width={400} height={30} fill="#f0f0f0" rx={2} />
      <image
        href="https://cdn.jsdelivr.net/npm/simple-icons@v11/icons/pagerduty.svg"
        x={40}
        y={10}
        width={14}
        height={14}
        opacity={0.3}
        transform="rotate(-5, 47, 17)"
      />
      <image
        href="https://cdn.jsdelivr.net/npm/simple-icons@v11/icons/opsgenie.svg"
        x={110}
        y={10}
        width={14}
        height={14}
        opacity={0.3}
        transform="rotate(5, 117, 17)"
      />
      <text
        x={200}
        y={24}
        fill={COLORS.textMuted}
        fontSize={7}
        textAnchor="middle"
      >
        $100k/year · per-seat pricing · vendor lock-in
      </text>

      {/* Right half — flowing/free */}
      <rect x={400} y={5} width={400} height={30} fill={COLORS.coral} fillOpacity={0.05} rx={2} />
      <image
        href="/logo-icon.png"
        x={550}
        y={8}
        width={16}
        height={16}
        opacity={0.7}
      />
      <text
        x={600}
        y={24}
        fill={COLORS.coral}
        fontSize={7}
        fontWeight={600}
        textAnchor="middle"
      >
        Free &amp; open source · self-hosted · your data
      </text>

      {/* Vertical divider */}
      <line x1={400} y1={5} x2={400} y2={35} stroke={COLORS.grayLight} />
    </g>
  )
}
