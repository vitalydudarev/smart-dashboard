import { ChevronDown, ChevronRight, Droplets, MapPin } from 'lucide-react'
import { useEffect, useId, useRef, useState } from 'react'
import { Link } from 'react-router'
import { HttpError } from '../../api/rates.ts'
import { type Coordinates, type StationForecast, type Weather, fetchStationForecast, fetchWeather } from '../../api/weather.ts'
import { formatDateTime, formatHumidity, formatTemperature, formatWeekdayDate } from '../../utils/format.ts'
import { currentPosition } from '../../utils/geolocation.ts'
import './WeatherWidget.scss'

const refreshInterval = 5 * 60 * 1000

type WeatherState = { status: 'loading' } | { status: 'error' } | { status: 'loaded'; weather: Weather }

type ForecastState =
  | { status: 'loading' }
  | { status: 'missing' }
  | { status: 'error' }
  | { status: 'loaded'; forecast: StationForecast }

const temperature = (celsius: number | null) => (celsius != null ? formatTemperature(celsius) : '—')

/** The station's forecast by day, fetched afresh each time it opens. */
function WidgetForecast({ stationId, id }: { stationId: number; id: string }) {
  const [state, setState] = useState<ForecastState>({ status: 'loading' })

  useEffect(() => {
    const controller = new AbortController()
    fetchStationForecast(stationId, controller.signal)
      .then((forecast) => setState({ status: 'loaded', forecast }))
      .catch((error: unknown) => {
        if (controller.signal.aborted) return
        if (error instanceof HttpError && error.status === 404) {
          setState({ status: 'missing' })
          return
        }
        console.error(error)
        setState({ status: 'error' })
      })
    return () => controller.abort()
  }, [stationId])

  return (
    <div id={id} className="weather-widget-forecast" aria-busy={state.status === 'loading'}>
      {state.status === 'loading' ? (
        <p className="weather-widget-forecast-message">Loading forecast…</p>
      ) : state.status === 'missing' || (state.status === 'loaded' && state.forecast.days.length === 0) ? (
        <p className="weather-widget-forecast-message">No forecast for this location</p>
      ) : state.status === 'error' ? (
        <p className="weather-widget-forecast-message weather-widget-warning" role="alert">
          Forecast is unavailable
        </p>
      ) : (
        <ol className="weather-widget-forecast-days">
          {state.forecast.days.map((day) => (
            <li key={day.date} className="weather-widget-forecast-day">
              <span className="weather-widget-forecast-when">
                <time dateTime={day.date}>{formatWeekdayDate(day.date)}</time>
                <span className="weather-widget-forecast-description">{day.description || '—'}</span>
              </span>
              <span className="weather-widget-forecast-temps">
                <span className="weather-widget-forecast-max">
                  <span className="app-visually-hidden">High </span>
                  {temperature(day.tempMax)}
                </span>
                <span className="weather-widget-forecast-min">
                  <span className="app-visually-hidden">Low </span>
                  {temperature(day.tempMin)}
                </span>
              </span>
            </li>
          ))}
        </ol>
      )}
    </div>
  )
}

/**
 * Current temperature and humidity at the Belhydromet station nearest the viewer, refreshed every 5 minutes;
 * opens the page with the weather at other stations. The button along the bottom drops down the station's forecast.
 */
function WeatherWidget() {
  const [state, setState] = useState<WeatherState>({ status: 'loading' })
  const [expanded, setExpanded] = useState(false)
  const widget = useRef<HTMLDivElement>(null)
  const forecastId = useId()

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

  // The forecast covers what is below the widget, so Escape or a click elsewhere folds it back.
  useEffect(() => {
    if (!expanded) return
    const onKeyDown = (event: KeyboardEvent) => {
      if (event.key === 'Escape') setExpanded(false)
    }
    const onPointerDown = (event: PointerEvent) => {
      if (!widget.current?.contains(event.target as Node)) setExpanded(false)
    }
    document.addEventListener('keydown', onKeyDown)
    document.addEventListener('pointerdown', onPointerDown)
    return () => {
      document.removeEventListener('keydown', onKeyDown)
      document.removeEventListener('pointerdown', onPointerDown)
    }
  }, [expanded])

  const weather = state.status === 'loaded' ? state.weather : null

  return (
    <div ref={widget} className={expanded ? 'weather-widget weather-widget--expanded' : 'weather-widget'}>
      <Link
        className="weather-widget-link"
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
      {/* Always there, so the card keeps its height while the weather loads. */}
      <button
        type="button"
        className="weather-widget-toggle"
        disabled={!weather}
        aria-expanded={expanded}
        aria-controls={expanded ? forecastId : undefined}
        onClick={() => setExpanded((open) => !open)}
      >
        Forecast
        <ChevronDown className="weather-widget-toggle-icon" size={16} aria-hidden="true" />
      </button>
      {expanded && weather && <WidgetForecast id={forecastId} stationId={weather.location.id} />}
    </div>
  )
}

export default WeatherWidget
