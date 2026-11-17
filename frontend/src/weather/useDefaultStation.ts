import { useSyncExternalStore } from 'react'

const STORAGE_KEY = 'default-weather-station'
// `storage` fires only in other tabs, so changes made here are announced with this event.
const CHANGE_EVENT = 'default-weather-station-change'

function read(): number | null {
  try {
    const stored = localStorage.getItem(STORAGE_KEY)
    const id = stored == null ? NaN : Number(stored)

    return Number.isInteger(id) ? id : null
  } catch {
    // Storage can be unavailable (private mode, blocked cookies): no saved station.
    return null
  }
}

function write(id: number | null) {
  try {
    if (id == null) localStorage.removeItem(STORAGE_KEY)
    else localStorage.setItem(STORAGE_KEY, String(id))
  } catch {
    // Ignore: the choice just will not survive a reload.
  }
  window.dispatchEvent(new Event(CHANGE_EVENT))
}

function subscribe(onChange: () => void) {
  const onStorage = (event: StorageEvent) => {
    if (event.key === STORAGE_KEY || event.key === null) onChange()
  }
  window.addEventListener('storage', onStorage)
  window.addEventListener(CHANGE_EVENT, onChange)

  return () => {
    window.removeEventListener('storage', onStorage)
    window.removeEventListener(CHANGE_EVENT, onChange)
  }
}

/**
 * The station the dashboard's weather widget shows, saved in localStorage; null means the station
 * nearest the viewer. Kept in sync across components and tabs.
 */
export function useDefaultStation() {
  const id = useSyncExternalStore(subscribe, read)

  return [id, write] as const
}
