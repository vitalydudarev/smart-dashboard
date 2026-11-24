import type { RateSource } from './rates.ts'
import { getJson } from './rates.ts'

export interface FootballLeague {
  id: number
  name: string
  country: string
  logo: string
}

export interface FootballSide {
  id: number
  name: string
  logo: string
  /** Null before kick-off. */
  goals: number | null
}

export interface FootballMatch {
  id: number
  leagueId: number
  /** e.g. "Regular Season - 13". */
  round: string
  /** ISO 8601 timestamp. */
  kickoff: string
  status: {
    /** API-Football's short code, e.g. NS, 1H, HT, FT. */
    short: string
    long: string
    /** Minutes played; null before kick-off. */
    elapsed: number | null
    extra: number | null
  }
  home: FootballSide
  away: FootballSide
  /** Empty when unknown. */
  venue: string
  /** Whether one of the dashboard's clubs plays. */
  featured: boolean
}

export interface FootballMatches {
  /** The leagues shown, in display order. */
  leagues: FootballLeague[]
  /** Every match of those leagues on the days covered, earliest first. */
  matches: FootballMatch[]
  /** The UTC days covered, as YYYY-MM-DD. */
  from: string
  to: string
  /** When the backend fetched the matches; scores and statuses are as of then. */
  updatedAt: string
  source: RateSource
}

/** Today's and tomorrow's matches of the shown leagues. The backend refreshes them at most hourly. */
export function fetchFootballMatches(signal?: AbortSignal): Promise<FootballMatches> {
  return getJson('/api/football/matches', {}, signal)
}
