import { ArrowLeft, Droplets, Search } from 'lucide-react'
import { useEffect, useRef, useState } from 'react'
import { Link } from 'react-router'
import { HttpError } from '../../api/rates.ts'
import {
  type Coordinates,
  type StationForecast,
  type Weather,
  type WeatherImage,
  type WeatherLocation,
  type WeatherStations,
  fetchRadarImage,
  fetchSatelliteImage,
  fetchStationForecast,
  fetchStationWeather,
  fetchWeatherStations,
} from '../../api/weather.ts'
import {
  formatDateTime,
  formatDistance,
  formatHumidity,
  formatTemperature,
  formatWeekdayDate,
} from '../../utils/format.ts'
import { currentPosition } from '../../utils/geolocation.ts'
import './WeatherPage.scss'

type StationsState =
  | { status: 'locating' }
  | { status: 'loading'; located: boolean }
  | { status: 'error'; message: string }
  | { status: 'loaded'; located: boolean; coords: Coordinates | null; result: WeatherStations }

type StationWeatherState =
  | { status: 'idle' }
  | { status: 'loading' }
  | { status: 'missing' }
  | { status: 'error' }
  | { status: 'loaded'; weather: Weather }

type ForecastState =
  | { status: 'loading' }
  | { status: 'missing' }
  | { status: 'error' }
  | { status: 'loaded'; forecast: StationForecast }

type ImageState =
  | { status: 'loading' }
  | { status: 'missing' }
  | { status: 'error' }
  | { status: 'loaded'; image: WeatherImage }

/** Calls back once, the first time the element scrolls near the viewport. */
function useFirstVisible<T extends Element>(onVisible: () => void) {
  const ref = useRef<T>(null)
  const callback = useRef(onVisible)
  useEffect(() => {
    callback.current = onVisible
  })

  useEffect(() => {
    const element = ref.current
    if (!element) return
    if (!('IntersectionObserver' in window)) {
      callback.current()
      return
    }
    const observer = new IntersectionObserver(
      (entries) => {
        if (entries.some((entry) => entry.isIntersecting)) {
          observer.disconnect()
          callback.current()
        }
      },
      { rootMargin: '200px 0px' },
    )
    observer.observe(element)
    return () => observer.disconnect()
  }, [])

  return ref
}

/**
 * One station; its weather loads when the row first scrolls into view, so a long list stays cheap.
 * Clicking the row shows its forecast.
 */
function StationRow({
  station,
  coords,
  selected,
  onSelect,
}: {
  station: WeatherLocation
  coords: Coordinates | null
  selected: boolean
  onSelect: (id: number) => void
}) {
  const [state, setState] = useState<StationWeatherState>({ status: 'idle' })
  const controller = useRef<AbortController | null>(null)

  const ref = useFirstVisible<HTMLLIElement>(() => {
    const current = new AbortController()
    controller.current = current
    setState({ status: 'loading' })
    fetchStationWeather(station.id, coords, current.signal)
      .then((weather) => setState({ status: 'loaded', weather }))
      .catch((error: unknown) => {
        if (current.signal.aborted) return
        if (error instanceof HttpError && error.status === 404) {
          setState({ status: 'missing' })
        } else {
          console.error(error)
          setState({ status: 'error' })
        }
      })
  })
  useEffect(() => () => controller.current?.abort(), [])

  const weather = state.status === 'loaded' ? state.weather : null

  return (
    <li ref={ref} className="weather-station" aria-busy={state.status === 'loading' || state.status === 'idle'}>
      <button
        type="button"
        className="weather-station-button"
        aria-pressed={selected}
        onClick={() => onSelect(station.id)}
      >
        <div className="weather-station-place">
          <span className="weather-station-name">
            {station.name}, {station.country}
          </span>
          <span className="weather-station-distance">{formatDistance(station.distanceKm)} away</span>
        </div>
        <div className="weather-station-reading">
          <span className="weather-station-temperature">{weather ? formatTemperature(weather.temperature) : '—'}</span>
          <span className="weather-station-detail">
            {weather ? (
              <>
                <span className="weather-station-humidity">
                  <Droplets size={14} aria-hidden="true" />
                  <span className="app-visually-hidden">Humidity </span>
                  {weather.humidity != null ? formatHumidity(weather.humidity) : '—'}
                </span>
                <time dateTime={weather.measuredAt}>{formatDateTime(weather.measuredAt)}</time>
              </>
            ) : state.status === 'missing' ? (
              'No current observation'
            ) : state.status === 'error' ? (
              <span className="weather-station-warning">Unavailable</span>
            ) : (
              'Loading…'
            )}
          </span>
        </div>
      </button>
    </li>
  )
}

/** The selected station's forecast by day, fetched afresh each time it mounts (no caching). */
function ForecastPanel({ station }: { station: WeatherLocation }) {
  const [state, setState] = useState<ForecastState>({ status: 'loading' })

  useEffect(() => {
    const controller = new AbortController()
    fetchStationForecast(station.id, controller.signal)
      .then((forecast) => setState({ status: 'loaded', forecast }))
      .catch((error: unknown) => {
        if (controller.signal.aborted) return
        if (error instanceof HttpError && error.status === 404) {
          setState({ status: 'missing' })
        } else {
          console.error(error)
          setState({ status: 'error' })
        }
      })
    return () => controller.abort()
  }, [station.id])

  const temperature = (celsius: number | null) => (celsius != null ? formatTemperature(celsius) : '—')

  return (
    <aside
      className="weather-forecast"
      aria-label={`Forecast for ${station.name}`}
      aria-busy={state.status === 'loading'}
    >
      <header className="weather-forecast-header">
        <h2>Forecast</h2>
        <span className="weather-forecast-place">
          {station.name}, {station.country}
        </span>
      </header>

      {state.status === 'loading' ? (
        <p className="weather-forecast-message">Loading forecast…</p>
      ) : state.status === 'missing' ? (
        <p className="weather-forecast-message">No forecast for this location</p>
      ) : state.status === 'error' ? (
        <p className="weather-forecast-message weather-station-warning" role="alert">
          Forecast is unavailable
        </p>
      ) : (
        <ol className="weather-forecast-days">
          {state.forecast.days.map((day) => (
            <li key={day.date} className="weather-forecast-day">
              <div className="weather-forecast-when">
                <time dateTime={day.date}>{formatWeekdayDate(day.date)}</time>
                <span className="weather-forecast-description">{day.description || '—'}</span>
              </div>
              <div className="weather-forecast-temps">
                <span className="weather-forecast-max">
                  <span className="app-visually-hidden">High </span>
                  {temperature(day.tempMax)}
                </span>
                <span className="weather-forecast-min">
                  <span className="app-visually-hidden">Low </span>
                  {temperature(day.tempMin)}
                </span>
              </div>
            </li>
          ))}
        </ol>
      )}
    </aside>
  )
}

/** The latest radar map or satellite image; clicking it opens the full-size image in a new tab. */
function WeatherImagePanel({ title, load }: { title: string; load: (signal: AbortSignal) => Promise<WeatherImage> }) {
  const [state, setState] = useState<ImageState>({ status: 'loading' })
  // The URL whose image failed to load in the browser, as opposed to the request for it.
  const [brokenUrl, setBrokenUrl] = useState<string | null>(null)

  useEffect(() => {
    const controller = new AbortController()
    load(controller.signal)
      .then((image) => setState({ status: 'loaded', image }))
      .catch((error: unknown) => {
        if (controller.signal.aborted) return
        if (error instanceof HttpError && error.status === 404) {
          setState({ status: 'missing' })
        } else {
          console.error(error)
          setState({ status: 'error' })
        }
      })
    return () => controller.abort()
  }, [load])

  const broken = state.status === 'loaded' && state.image.url === brokenUrl

  return (
    <section className="weather-image" aria-label={title} aria-busy={state.status === 'loading'}>
      <h2>{title}</h2>
      {state.status === 'loading' ? (
        <p className="weather-forecast-message">Loading image…</p>
      ) : state.status === 'missing' ? (
        <p className="weather-forecast-message">No image available</p>
      ) : state.status === 'error' || broken ? (
        <p className="weather-forecast-message weather-station-warning" role="alert">
          Image is unavailable
        </p>
      ) : (
        <a
          className="weather-image-link"
          href={state.image.url}
          target="_blank"
          rel="noopener noreferrer"
          title="Open full size"
        >
          <img src={state.image.url} alt={title} loading="lazy" onError={() => setBrokenUrl(state.image.url)} />
        </a>
      )}
    </section>
  )
}

/** The weather at every Belhydromet station, nearest to the viewer first. */
function WeatherPage() {
  const [state, setState] = useState<StationsState>({ status: 'locating' })
  const [filter, setFilter] = useState('')
  const [selectedId, setSelectedId] = useState<number | null>(null)

  useEffect(() => {
    let disposed = false
    const controller = new AbortController()

    void currentPosition().then((coords) => {
      if (disposed) return
      const located = coords !== null
      setState({ status: 'loading', located })
      fetchWeatherStations(coords, controller.signal)
        .then((result) => setState({ status: 'loaded', located, coords, result }))
        .catch((error: unknown) => {
          if (controller.signal.aborted) return
          console.error(error)
          setState({ status: 'error', message: error instanceof Error ? error.message : 'Weather is unavailable' })
        })
    })

    return () => {
      disposed = true
      controller.abort()
    }
  }, [])

  const query = filter.trim().toLocaleLowerCase()
  const stations =
    state.status === 'loaded'
      ? state.result.stations.filter(
          (station) =>
            !query ||
            station.name.toLocaleLowerCase().includes(query) ||
            station.country.toLocaleLowerCase().includes(query),
        )
      : []
  // The first row until the viewer picks one, or when the filter hides the one they picked.
  const selected = stations.find((station) => station.id === selectedId) ?? stations[0]

  return (
    <div className="weather-page">
      <Link className="weather-back" to="/">
        <ArrowLeft size={16} aria-hidden="true" />
        Dashboard
      </Link>

      <header className="weather-header">
        <div>
          <h1>Weather</h1>
          {(state.status === 'loading' || state.status === 'loaded') && (
            <p className="weather-subtitle">
              {state.located
                ? 'Stations sorted by distance from you'
                : 'Your location is unavailable, so stations are sorted by distance from Minsk'}
            </p>
          )}
        </div>
        <label className="weather-search">
          <Search className="weather-search-icon" size={16} aria-hidden="true" />
          <span className="app-visually-hidden">Find a location</span>
          <input
            type="search"
            placeholder="Find a location"
            value={filter}
            onChange={(event) => setFilter(event.target.value)}
            disabled={state.status !== 'loaded'}
          />
        </label>
      </header>

      {state.status === 'locating' || state.status === 'loading' ? (
        <p className="weather-message">{state.status === 'locating' ? 'Finding your location…' : 'Loading stations…'}</p>
      ) : state.status === 'error' ? (
        <p className="weather-message weather-station-warning" role="alert">
          {state.message}
        </p>
      ) : stations.length === 0 ? (
        <p className="weather-message">No location matches “{filter.trim()}”.</p>
      ) : (
        <>
          <div className="weather-content">
            <ul className="weather-stations" aria-label="Weather stations, nearest first">
              {stations.map((station) => (
                <StationRow
                  key={station.id}
                  station={station}
                  coords={state.coords}
                  selected={station.id === selected.id}
                  onSelect={setSelectedId}
                />
              ))}
            </ul>
            <div className="weather-sidebar">
              {/* Keyed by station so each selection starts from a fresh "loading" state. */}
              <ForecastPanel key={selected.id} station={selected} />
              <WeatherImagePanel title="Radar" load={fetchRadarImage} />
              <WeatherImagePanel title="Satellite" load={fetchSatelliteImage} />
            </div>
          </div>
          <p className="weather-source">Source: {state.result.source.name}</p>
        </>
      )}
    </div>
  )
}

export default WeatherPage
