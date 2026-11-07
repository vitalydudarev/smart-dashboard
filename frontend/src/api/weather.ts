import type { RateSource } from './rates.ts'
import { getJson } from './rates.ts'

/** The weather station the observation comes from. */
export interface WeatherLocation {
  id: number
  name: string
  /** Country code, e.g. BY. */
  country: string
  /** Distance from the requested coordinates. */
  distanceKm: number
}

export interface Weather {
  location: WeatherLocation
  /** Degrees Celsius. */
  temperature: number
  /** Relative humidity in percent; null when the station did not report it. */
  humidity: number | null
  /** When the station took the measurement, as an ISO 8601 timestamp. */
  measuredAt: string
  source: RateSource
}

export interface Coordinates {
  lat: number
  lng: number
}

/** The latest observation at the nearest working station; the backend picks Minsk without coordinates. */
export function fetchWeather(coords: Coordinates | null, signal?: AbortSignal): Promise<Weather> {
  const params: Record<string, string> = coords ? { lat: String(coords.lat), lng: String(coords.lng) } : {}
  return getJson('/api/weather', params, signal)
}
