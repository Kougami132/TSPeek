import type { PublicConfig } from '../types'

export async function fetchPublicConfig(): Promise<PublicConfig> {
  const response = await fetch('/api/v1/public-config', { cache: 'no-store' })
  return response.json()
}
