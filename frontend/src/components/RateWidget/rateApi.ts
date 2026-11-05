export interface ExchangeRate {
  pair: string
  rate: number
  /** The day the rate was loaded for, as YYYY-MM-DD. */
  date: string
}

export async function fetchEurUsdRate(signal?: AbortSignal): Promise<ExchangeRate> {
  const response = await fetch('/api/rates/eur-usd', { signal })
  if (!response.ok) {
    throw new Error(`Rate request failed with status ${response.status}`)
  }
  return response.json() as Promise<ExchangeRate>
}
