export interface Currency {
  code: string
  name: string
}

export interface LatestRate {
  base: string
  quote: string
  rate: number
  /** The day the rate was loaded for, as YYYY-MM-DD. */
  date: string
}

export interface RatePoint {
  /** YYYY-MM-DD */
  date: string
  rate: number
}

export interface RateHistory {
  base: string
  quote: string
  /** The period actually returned, after the backend clamps it to the days it has data for. */
  from: string
  to: string
  points: RatePoint[]
}

export type RangePreset = '7d' | '30d' | '6m' | '1y' | '5y'

/** Either a preset counted back from the latest day, or explicit YYYY-MM-DD bounds. */
export type RatePeriod = { range: RangePreset } | { from: string; to: string }

async function getJson<T>(path: string, params: Record<string, string>, signal?: AbortSignal): Promise<T> {
  const query = new URLSearchParams(params).toString()
  const response = await fetch(query ? `${path}?${query}` : path, { signal })

  if (!response.ok) {
    // The backend explains bad requests as {"error": "..."}; anything else gets a generic message.
    const body = (await response.json().catch(() => null)) as { error?: string } | null
    throw new Error(body?.error ?? `Request failed with status ${response.status}`)
  }

  return response.json() as Promise<T>
}

export function fetchCurrencies(signal?: AbortSignal): Promise<Currency[]> {
  return getJson('/api/currencies', {}, signal)
}

export function fetchLatestRate(base: string, quote: string, signal?: AbortSignal): Promise<LatestRate> {
  return getJson('/api/rates/latest', { base, quote }, signal)
}

export function fetchRateHistory(
  base: string,
  quote: string,
  period: RatePeriod,
  signal?: AbortSignal,
): Promise<RateHistory> {
  return getJson('/api/rates/history', { base, quote, ...period }, signal)
}
