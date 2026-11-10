import L from 'leaflet'
import 'leaflet/dist/leaflet.css'
import { useEffect, useRef, useState } from 'react'
import { type WeatherMap as WeatherMapData, type WeatherMapStation, fetchWeatherMap } from '../../api/weather.ts'
import { formatDateTime, formatHumidity, formatTemperature } from '../../utils/format.ts'
import './WeatherMap.scss'

/** The country whose stations the map shows. */
const COUNTRY = 'BY'

type MapState =
  | { status: 'loading' }
  | { status: 'error'; message: string }
  | { status: 'loaded'; map: WeatherMapData }

/** Hue by temperature: blues below freezing, green around 10 °C, orange and red in the heat. */
const HUE_STOPS: [celsius: number, hue: number][] = [
  [-20, 250],
  [-10, 225],
  [0, 195],
  [10, 120],
  [20, 35],
  [30, 0],
]

function temperatureColor(celsius: number): string {
  const upper = HUE_STOPS.findIndex(([at]) => celsius <= at)
  let hue: number
  if (upper === 0) hue = HUE_STOPS[0][1]
  else if (upper < 0) hue = HUE_STOPS[HUE_STOPS.length - 1][1]
  else {
    const [fromAt, fromHue] = HUE_STOPS[upper - 1]
    const [toAt, toHue] = HUE_STOPS[upper]
    hue = fromHue + ((celsius - fromAt) / (toAt - fromAt)) * (toHue - fromHue)
  }
  return `hsl(${Math.round(hue)} 70% 42%)`
}

/** The marker: the rounded temperature on a disc coloured by it, or a grey dot without a reading. */
function stationIcon(station: WeatherMapStation): L.DivIcon {
  const element = document.createElement('span')
  if (station.temperature == null) {
    element.className = 'weather-map-marker weather-map-marker--empty'
    return L.divIcon({ html: element, className: '', iconSize: [10, 10] })
  }
  element.className = 'weather-map-marker'
  element.style.background = temperatureColor(station.temperature)
  // Math.round(-0.4) is -0, which would print as "-0".
  const label = `${Math.round(station.temperature) || 0}°`
  element.textContent = label
  // Room for "-12°" as well as "4°".
  return L.divIcon({ html: element, className: '', iconSize: [label.length > 3 ? 42 : 34, 22] })
}

/** The popup: built from DOM nodes so station names are never parsed as HTML. */
function stationPopup(station: WeatherMapStation): HTMLElement {
  const popup = document.createElement('div')
  popup.className = 'weather-map-popup'
  const line = (className: string, text: string) => {
    const element = document.createElement('div')
    element.className = className
    element.textContent = text
    popup.append(element)
  }
  line('weather-map-popup-name', station.name)
  if (station.temperature == null) {
    line('weather-map-popup-detail', 'No current reading')
  } else {
    line('weather-map-popup-temperature', formatTemperature(station.temperature))
    if (station.humidity != null) line('weather-map-popup-detail', `Humidity ${formatHumidity(station.humidity)}`)
    if (station.measuredAt) line('weather-map-popup-detail', formatDateTime(station.measuredAt))
  }
  return popup
}

/** An OpenStreetMap map of every Belarusian station's latest temperature. */
function WeatherMap() {
  const [state, setState] = useState<MapState>({ status: 'loading' })
  const container = useRef<HTMLDivElement>(null)

  useEffect(() => {
    const controller = new AbortController()
    fetchWeatherMap(COUNTRY, controller.signal)
      .then((map) => setState({ status: 'loaded', map }))
      .catch((error: unknown) => {
        if (controller.signal.aborted) return
        console.error(error)
        setState({ status: 'error', message: error instanceof Error ? error.message : 'Weather is unavailable' })
      })
    return () => controller.abort()
  }, [])

  const stations = state.status === 'loaded' ? state.map.stations : null

  useEffect(() => {
    if (!container.current || !stations) return
    const map = L.map(container.current, { zoomSnap: 0.5 })
    L.tileLayer('https://tile.openstreetmap.org/{z}/{x}/{y}.png', {
      maxZoom: 19,
      attribution: '&copy; <a href="https://www.openstreetmap.org/copyright">OpenStreetMap</a> contributors',
    }).addTo(map)

    // Stations with a reading are drawn last, on top of the grey dots.
    const ordered = [...stations].sort((a, b) => Number(a.temperature != null) - Number(b.temperature != null))
    for (const station of ordered) {
      L.marker([station.lat, station.lng], {
        icon: stationIcon(station),
        title: station.name,
        alt: station.temperature == null ? station.name : `${station.name}, ${formatTemperature(station.temperature)}`,
        riseOnHover: true,
      })
        .bindPopup(() => stationPopup(station))
        .addTo(map)
    }

    if (stations.length > 0) {
      map.fitBounds(L.latLngBounds(stations.map((station) => [station.lat, station.lng])), { padding: [24, 24] })
    } else {
      map.setView([53.9, 27.56], 6)
    }

    // The container can change size with the window or the page layout.
    const resize = new ResizeObserver(() => map.invalidateSize())
    resize.observe(container.current)
    return () => {
      resize.disconnect()
      map.remove()
    }
  }, [stations])

  if (state.status === 'loading') return <p className="weather-message">Loading stations…</p>
  if (state.status === 'error') {
    return (
      <p className="weather-message weather-station-warning" role="alert">
        {state.message}
      </p>
    )
  }

  const reporting = state.map.stations.filter((station) => station.temperature != null).length
  return (
    <>
      <div ref={container} className="weather-map" role="region" aria-label="Map of station temperatures" />
      <p className="weather-source">
        {reporting} of {state.map.stations.length} stations in Belarus reporting · Source: {state.map.source.name}
      </p>
    </>
  )
}

export default WeatherMap
