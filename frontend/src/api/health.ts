export interface HealthResponse {
  status: string
  version: string
}

export async function getHealth(): Promise<HealthResponse> {
  const res = await fetch('/health')
  return res.json() as Promise<HealthResponse>
}
