export interface ExchangeRate {
  pair: string
  rate: number
  /** The day the rate was loaded for, as YYYY-MM-DD. */
  date: string
}

// Placeholder until the backend provides real rates.
export const mockRate: ExchangeRate = {
  pair: 'EUR/USD',
  rate: 1.0842,
  date: '2026-11-02',
}
