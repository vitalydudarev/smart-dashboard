import L from 'leaflet'
import 'leaflet/dist/leaflet.css'
import { useEffect, useRef, useState } from 'react'
import { type WeatherMap as WeatherMapData, type WeatherMapStation, fetchWeatherMap } from '../../api/weather.ts'
import { formatDateTime, formatHumidity, formatSignedTemperature } from '../../utils/format.ts'
import './WeatherMap.scss'

/** Space between the outermost stations and the map's edges: about half a temperature badge. */
const MAP_PADDING = 144

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

/** The marker: the temperature as "+X.Y" on a disc coloured by it, or a grey dot without a reading. */
function stationIcon(station: WeatherMapStation): L.DivIcon {
  const element = document.createElement('span')
  if (station.temperature == null) {
    element.className = 'weather-map-marker weather-map-marker--empty'
    return L.divIcon({ html: element, className: '', iconSize: [10, 10] })
  }
  element.className = 'weather-map-marker'
  element.style.background = temperatureColor(station.temperature)
  const label = formatSignedTemperature(station.temperature)
  element.textContent = label
  // Room for "-12.3" as well as "+0.9".
  return L.divIcon({ html: element, className: '', iconSize: [label.length > 4 ? 46 : 40, 22] })
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
    line('weather-map-popup-temperature', formatSignedTemperature(station.temperature))
    if (station.humidity != null) line('weather-map-popup-detail', `Humidity ${formatHumidity(station.humidity)}`)
    if (station.measuredAt) line('weather-map-popup-detail', formatDateTime(station.measuredAt))
  }
  return popup
}

/** Zoom levels per wheel pixel: trackpad pinches (sent as ctrl+wheel) arrive as many small deltas. */
const PINCH_ZOOM_RATE = 1 / 100
/** Zoom levels per wheel pixel for a mouse wheel or a two-finger trackpad scroll. */
const SCROLL_ZOOM_RATE = 1 / 200
const LINE_HEIGHT_PX = 16

/** Safari's desktop pinch event; not in the DOM typings. */
type GestureEvent = UIEvent & { scale: number; clientX: number; clientY: number }

/**
 * Zooms the map with the wheel and trackpad pinches, following every delta immediately around
 * the pointer. Chrome and Firefox report pinches as ctrl+wheel; Safari as gesture events.
 * Returns a function that removes the listeners.
 */
function followWheelZoom(map: L.Map, element: HTMLElement): () => void {
  const clampZoom = (zoom: number) => Math.min(map.getMaxZoom(), Math.max(map.getMinZoom(), zoom))
  const zoomAround = (event: { clientX: number; clientY: number }, zoom: number) => {
    const target = clampZoom(zoom)
    if (target === map.getZoom()) return
    map.setZoomAround(map.mouseEventToContainerPoint(event as MouseEvent), target, { animate: false })
  }

  // Deltas are summed and applied once per frame, so a burst of events costs one redraw.
  let pending = 0
  let pointer = { clientX: 0, clientY: 0 }
  let frame = 0
  const onWheel = (event: WheelEvent) => {
    event.preventDefault()
    const pixels = event.deltaMode === WheelEvent.DOM_DELTA_LINE ? event.deltaY * LINE_HEIGHT_PX : event.deltaY
    // A single mouse-wheel notch moves at most one level.
    pending += Math.max(-1, Math.min(1, -pixels * (event.ctrlKey ? PINCH_ZOOM_RATE : SCROLL_ZOOM_RATE)))
    pointer = { clientX: event.clientX, clientY: event.clientY }
    if (frame) return
    frame = requestAnimationFrame(() => {
      frame = 0
      map.stop()
      zoomAround(pointer, map.getZoom() + pending)
      pending = 0
    })
  }

  let gestureStartZoom = 0
  const onGestureStart = (event: Event) => {
    event.preventDefault()
    map.stop()
    gestureStartZoom = map.getZoom()
  }
  const onGestureChange = (event: Event) => {
    event.preventDefault()
    const gesture = event as GestureEvent
    zoomAround(gesture, gestureStartZoom + Math.log2(gesture.scale))
  }

  element.addEventListener('wheel', onWheel, { passive: false })
  element.addEventListener('gesturestart', onGestureStart)
  element.addEventListener('gesturechange', onGestureChange)
  return () => {
    cancelAnimationFrame(frame)
    element.removeEventListener('wheel', onWheel)
    element.removeEventListener('gesturestart', onGestureStart)
    element.removeEventListener('gesturechange', onGestureChange)
  }
}

/** An OpenStreetMap map of every Belarusian station's latest temperature. */
function WeatherMap() {
  const [state, setState] = useState<MapState>({ status: 'loading' })
  const container = useRef<HTMLDivElement>(null)

  useEffect(() => {
    const controller = new AbortController()
    fetchWeatherMap(controller.signal)
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
    // zoomSnap 0 lets fitBounds pick a fractional zoom, so the outermost stations sit right at the edges.
    // Leaflet's own wheel zoom debounces and animates each tiny trackpad delta, which makes pinching
    // crawl; it is replaced by followWheelZoom below.
    const map = L.map(container.current, { zoomSnap: 0, zoomDelta: 0.5, scrollWheelZoom: false })
    const stopWheelZoom = followWheelZoom(map, container.current)
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
        alt: station.temperature == null ? station.name : `${station.name}, ${formatSignedTemperature(station.temperature)} °C`,
        riseOnHover: true,
      })
        .bindPopup(() => stationPopup(station))
        .addTo(map)
    }

    const bounds = stations.length > 0 ? L.latLngBounds(stations.map((station) => [station.lat, station.lng])) : null

    // As close as all the stations fit: the outermost ones reach the edges in whichever direction
    // is tighter, and the map shows more of the surroundings in the other. The padding leaves room
    // for half a temperature badge beyond the coordinate.
    const fit = () => {
      if (bounds) map.fitBounds(bounds, { padding: [MAP_PADDING, MAP_PADDING], animate: false })
      else map.setView([53.9, 27.56], 6, { animate: false })
    }
    fit()

    // The container can change size with the window or the page layout; keep the stations fitted.
    const resize = new ResizeObserver(() => {
      map.invalidateSize({ animate: false })
      fit()
    })
    resize.observe(container.current)
    return () => {
      resize.disconnect()
      stopWheelZoom()
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
