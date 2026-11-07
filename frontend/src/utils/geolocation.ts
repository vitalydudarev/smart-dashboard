import type { Coordinates } from '../api/weather.ts'

/** The viewer's position, or null when the browser cannot or may not tell; never rejects. */
export function currentPosition(): Promise<Coordinates | null> {
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
