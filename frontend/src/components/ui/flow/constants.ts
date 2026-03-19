export const COLORS = {
  coral: '#f06292',
  coralDeep: '#b02b52',
  coralLight: '#f8a4c0',
  purple: '#7b1fa2',
  amber: '#ff8f00',
  blue: '#1976d2',
  green: '#43a047',
  red: '#e53935',
  gray: '#9e9e9e',
  grayLight: '#e0e0e0',
  text: '#374151',
  textMuted: '#9ca3af',
  white: '#ffffff',
  bg: '#fafafa',
} as const

export const LOOP_DURATION = 12

export const CANVAS = {
  width: 800,
  height: 400,
} as const

export const ZONES = {
  sources: { x: 0, width: 180 },
  nexus: { x: 180, width: 260 },
  ai: { x: 440, width: 240 },
  resolution: { x: 680, width: 120 },
} as const
