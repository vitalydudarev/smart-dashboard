import { useEffect, useState } from 'react'
import { type FootballMatch, type FootballMatches, fetchFootballMatches } from '../api/football.ts'

export type FootballState =
  | { status: 'loading' }
  | { status: 'error' }
  | { status: 'loaded'; data: FootballMatches }

/** The API-Football free plan allows 100 requests a day, so the backend refreshes hourly and so do we. */
const REFRESH_MS = 60 * 60 * 1000

/** The matches, loaded when the component mounts (so on every page load) and again every hour. */
export function useFootballMatches(): FootballState {
  const [state, setState] = useState<FootballState>({ status: 'loading' })

  useEffect(() => {
    let controller = new AbortController()
    const load = () => {
      controller.abort()
      controller = new AbortController()
      const { signal } = controller
      fetchFootballMatches(signal)
        .then((data) => setState({ status: 'loaded', data }))
        .catch((error: unknown) => {
          if (signal.aborted) return
          console.error(error)
          // Keep showing what we have when a later refresh fails.
          setState((previous) => (previous.status === 'loaded' ? previous : { status: 'error' }))
        })
    }
    load()
    const timer = window.setInterval(load, REFRESH_MS)
    return () => {
      window.clearInterval(timer)
      controller.abort()
    }
  }, [])

  return state
}

const LIVE = new Set(['1H', 'HT', '2H', 'ET', 'BT', 'P', 'SUSP', 'INT', 'LIVE'])
const FINISHED = new Set(['FT', 'AET', 'PEN'])
const UPCOMING = new Set(['NS', 'TBD'])

export type MatchPhase = 'live' | 'finished' | 'upcoming' | 'other'

/** Postponed, cancelled, abandoned and awarded matches are 'other'. */
export function matchPhase(match: FootballMatch): MatchPhase {
  const code = match.status.short
  if (LIVE.has(code)) return 'live'
  if (FINISHED.has(code)) return 'finished'
  if (UPCOMING.has(code)) return 'upcoming'
  return 'other'
}

const time = new Intl.DateTimeFormat('en-GB', { hour: '2-digit', minute: '2-digit' })
const weekdayDate = new Intl.DateTimeFormat('en-GB', { weekday: 'short', day: 'numeric', month: 'short' })

const localDay = (date: Date) => new Date(date.getFullYear(), date.getMonth(), date.getDate()).getTime()

/** The kick-off day in the viewer's zone: "Today", "Tomorrow", "Yesterday" or e.g. "Thu 26 Nov". */
export function formatMatchDay(iso: string): string {
  const days = Math.round((localDay(new Date(iso)) - localDay(new Date())) / 86_400_000)
  if (days === 0) return 'Today'
  if (days === 1) return 'Tomorrow'
  if (days === -1) return 'Yesterday'
  return weekdayDate.format(new Date(iso))
}

/** The kick-off time in the viewer's zone, e.g. "20:00". */
export const formatKickoffTime = (iso: string) => time.format(new Date(iso))

/** A live match's clock: "HT", "61'", "90+3'", or the status code (e.g. "P" for penalties). */
export function formatMatchClock(match: FootballMatch): string {
  const { short, elapsed, extra } = match.status
  if (short === 'HT' || short === 'BT' || short === 'P' || short === 'SUSP' || short === 'INT') return short
  if (elapsed == null) return 'Live'
  return extra ? `${elapsed}+${extra}'` : `${elapsed}'`
}
