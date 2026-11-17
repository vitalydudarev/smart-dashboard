import { useEffect, useState } from 'react'
import { HttpError } from '../api/rates.ts'
import { type Coordinates, type Weather, fetchStationWeather, fetchWeather } from '../api/weather.ts'
import { currentPosition } from '../utils/geolocation.ts'
import { useDefaultStation } from './useDefaultStation.ts'

const refreshInterval = 5 * 60 * 1000

export type ShownWeatherState = { status: 'loading' } | { status: 'error' } | { status: 'loaded'; weather: Weather }

/**
 * The latest weather at the station starred on the weather page, or else at the station nearest the viewer
 * (also the fallback while the starred one has no observation), refreshed every 5 minutes.
 */
export function useShownWeather(): ShownWeatherState {
  const [state, setState] = useState<ShownWeatherState>({ status: 'loading' })
  const [defaultId] = useDefaultStation()

  useEffect(() => {
    let controller: AbortController | undefined
    let position: Promise<Coordinates | null> | undefined

    // Asks for the viewer's location only when the nearest station is needed, and only once.
    const nearest = async (signal: AbortSignal) => fetchWeather(await (position ??= currentPosition()), signal)

    const fetchShown = (signal: AbortSignal) =>
      defaultId == null
        ? nearest(signal)
        : fetchStationWeather(defaultId, null, signal).catch((error: unknown) => {
            if (error instanceof HttpError && error.status === 404) return nearest(signal)
            throw error
          })

    const load = () => {
      controller?.abort()
      const current = new AbortController()
      controller = current
      fetchShown(current.signal)
        .then((weather) => {
          if (!current.signal.aborted) setState({ status: 'loaded', weather })
        })
        .catch((error: unknown) => {
          if (!current.signal.aborted) {
            console.error(error)
            setState({ status: 'error' })
          }
        })
    }

    load()
    const timer = setInterval(load, refreshInterval)

    return () => {
      controller?.abort()
      clearInterval(timer)
    }
  }, [defaultId])

  return state
}
