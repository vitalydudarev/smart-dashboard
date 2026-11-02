import { useEffect, useState } from 'react'

export type ThemePreference = 'system' | 'light' | 'dark'

// The inline script in index.html reads this key too.
const STORAGE_KEY = 'theme-preference'

function isThemePreference(value: unknown): value is ThemePreference {
  return value === 'system' || value === 'light' || value === 'dark'
}

function readPreference(): ThemePreference {
  try {
    const stored = localStorage.getItem(STORAGE_KEY)

    return isThemePreference(stored) ? stored : 'system'
  } catch {
    // Storage can be unavailable (private mode, blocked cookies); the default is fine.
    return 'system'
  }
}

function writePreference(preference: ThemePreference) {
  try {
    localStorage.setItem(STORAGE_KEY, preference)
  } catch {
    // Ignore: the preference just will not survive a reload.
  }
}

/**
 * Writes the chosen theme to `data-theme` on `<html>`; `src/styles/_tokens.scss` reads it.
 * 'system' removes the attribute so the `prefers-color-scheme` media query takes over.
 */
export function useThemePreference() {
  const [preference, setPreference] = useState(readPreference)

  useEffect(() => {
    const root = document.documentElement

    if (preference === 'system') {
      root.removeAttribute('data-theme')
    } else {
      root.setAttribute('data-theme', preference)
    }

    writePreference(preference)
  }, [preference])

  return [preference, setPreference] as const
}
