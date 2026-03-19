export interface AnimationPhase {
  zone: 'sources' | 'nexus' | 'ai' | 'resolution'
  startTime: number
  endTime: number
}

export interface Position {
  x: number
  y: number
}
