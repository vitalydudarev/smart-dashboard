import { ChevronRight, Droplets, MapPin } from 'lucide-react'
import { useEffect, useState } from 'react'
import { Link } from 'react-router'
import { type Coordinates, type Weather, fetchWeather } from '../../api/weather.ts'
import { formatDateTime, formatHumidity, formatTemperature } from '../../utils/format.ts'
import { currentPosition } from '../../utils/geolocation.ts'
import './WeatherWidget.scss'

const refreshInterval = 5 * 60 * 1000

type WeatherState = { status: 'loading' } | { status: 'error' } | { status: 'loaded'; weather: Weather }

/**
 * Current temperature and humidity at the Belhydromet station nearest the viewer, refreshed every 5 minutes;
 * opens the page with the weather at other stations.
 */
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
    <Link
      className="weather-widget"
      to="/weather"
      aria-label={`${weather ? `Weather in ${weather.location.name}` : 'Current weather'}. View other locations`}
      aria-busy={state.status === 'loading'}
    >
      <span className="weather-widget-location">
        <MapPin className="weather-widget-icon" size={16} aria-hidden="true" />
        <span className="weather-widget-location-name">
          {weather ? `${weather.location.name}, ${weather.location.country}` : 'Weather'}
        </span>
        <ChevronRight className="weather-widget-chevron" size={18} aria-hidden="true" />
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
    </Link>
  )
}

export default WeatherWidget
