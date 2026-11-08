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

export interface WeatherStations {
  /** Every station, nearest to the requested coordinates first. */
  stations: WeatherLocation[]
  source: RateSource
}

const coordsParams = (coords: Coordinates | null): Record<string, string> =>
  coords ? { lat: String(coords.lat), lng: String(coords.lng) } : {}

/** The latest observation at the nearest working station; the backend picks Minsk without coordinates. */
export function fetchWeather(coords: Coordinates | null, signal?: AbortSignal): Promise<Weather> {
  return getJson('/api/weather', coordsParams(coords), signal)
}

/** Every station, nearest first; distances are from Minsk without coordinates. */
export function fetchWeatherStations(coords: Coordinates | null, signal?: AbortSignal): Promise<WeatherStations> {
  return getJson('/api/weather/stations', coordsParams(coords), signal)
}

/** The latest observation at one station. Rejects with a 404 HttpError when it has none. */
export function fetchStationWeather(id: number, coords: Coordinates | null, signal?: AbortSignal): Promise<Weather> {
  return getJson(`/api/weather/stations/${id}`, coordsParams(coords), signal)
}

export interface ForecastDay {
  /** The day, as YYYY-MM-DD. */
  date: string
  /** Degrees Celsius; null when the forecast has none. */
  tempMin: number | null
  tempMax: number | null
  /** The midday weather as Belhydromet words it, e.g. "Облачно"; empty when unknown. */
  description: string
}

export interface StationForecast {
  stationId: number
  /** Earliest day first. */
  days: ForecastDay[]
  source: RateSource
}

/** One station's forecast by day. Never cached. Rejects with a 404 HttpError when the station has none. */
export function fetchStationForecast(id: number, signal?: AbortSignal): Promise<StationForecast> {
  return getJson(`/api/weather/stations/${id}/forecast`, {}, signal)
}

/** The latest radar map or satellite image. */
export interface WeatherImage {
  /** The image itself, on pogoda.by. */
  url: string
  source: RateSource
}

/** The Minsk radar's latest map. Never cached. Rejects with a 404 HttpError when there is none. */
export function fetchRadarImage(signal?: AbortSignal): Promise<WeatherImage> {
  return getJson('/api/weather/radar', {}, signal)
}

/** The latest satellite image. Never cached. Rejects with a 404 HttpError when there is none. */
export function fetchSatelliteImage(signal?: AbortSignal): Promise<WeatherImage> {
  return getJson('/api/weather/satellite', {}, signal)
}
