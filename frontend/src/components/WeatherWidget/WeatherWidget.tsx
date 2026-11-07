import { Droplets, MapPin } from 'lucide-react'
import { useEffect, useState } from 'react'
import { type Coordinates, type Weather, fetchWeather } from '../../api/weather.ts'
import { formatDateTime, formatHumidity, formatTemperature } from '../../utils/format.ts'
import './WeatherWidget.scss'

const refreshInterval = 5 * 60 * 1000

type WeatherState = { status: 'loading' } | { status: 'error' } | { status: 'loaded'; weather: Weather }

/** The viewer's position, or null when the browser cannot or may not tell; never rejects. */
function currentPosition(): Promise<Coordinates | null> {
  return new Promise((resolve) => {
    if (!('geolocation' in navigator)) {
      resolve(null)
      return
    }
    navigator.geolocation.getCurrentPosition(
      ({ coords }) => resolve({ lat: coords.latitude, lng: coords.longitude }),
      (error) => {
        console.warn('Geolocation unavailable, using the default location:', error.message)
        resolve(null)
      },
      // A city-level fix is plenty for picking a weather station.
      { enableHighAccuracy: false, timeout: 10_000, maximumAge: 30 * 60 * 1000 },
    )
  })
}

/** Current temperature and humidity at the Belhydromet station nearest the viewer, refreshed every 5 minutes. */
function WeatherWidget() {
  const [state, setState] = useState<WeatherState>({ status: 'loading' })

  useEffect(() => {
    let disposed = false
    let controller: AbortController | undefined
    let timer: ReturnType<typeof setInterval> | undefined

    const load = (coords: Coordinates | null) => {
      controller?.abort()
      const current = new AbortController()
      controller = current
      fetchWeather(coords, current.signal)
        .then((weather) => setState({ status: 'loaded', weather }))
        .catch((error: unknown) => {
          if (!current.signal.aborted) {
            console.error(error)
            setState({ status: 'error' })
          }
        })
    }

    void currentPosition().then((coords) => {
      if (disposed) return
      load(coords)
      timer = setInterval(() => load(coords), refreshInterval)
    })

    return () => {
      disposed = true
      controller?.abort()
      clearInterval(timer)
    }
  }, [])

  const weather = state.status === 'loaded' ? state.weather : null

  return (
    <section className="weather-widget" aria-label="Current weather" aria-busy={state.status === 'loading'}>
      <span className="weather-widget-location">
        <MapPin className="weather-widget-icon" size={16} aria-hidden="true" />
        {weather ? `${weather.location.name}, ${weather.location.country}` : 'Weather'}
      </span>
      <span className="weather-widget-temperature">{weather ? formatTemperature(weather.temperature) : '—'}</span>
      <span className="weather-widget-humidity">
        <Droplets className="weather-widget-icon" size={16} aria-hidden="true" />
        <span className="app-visually-hidden">Humidity </span>
        {weather?.humidity != null ? formatHumidity(weather.humidity) : '—'}
      </span>
      {weather ? (
        <time className="weather-widget-date" dateTime={weather.measuredAt}>
          Measured {formatDateTime(weather.measuredAt)}
        </time>
      ) : state.status === 'loading' ? (
        <span className="weather-widget-date">Loading…</span>
      ) : (
        <span className="weather-widget-date weather-widget-warning" role="status">
          Weather is currently unavailable
        </span>
      )}
    </section>
  )
}

export default WeatherWidget
