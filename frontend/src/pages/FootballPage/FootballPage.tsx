import { ArrowLeft } from 'lucide-react'
import { Link } from 'react-router'
import type { FootballMatch } from '../../api/football.ts'
import MatchRow from '../../football/MatchRow.tsx'
import { formatKickoffTime, formatMatchDay, useFootballMatches } from '../../football/useFootballMatches.ts'
import './FootballPage.scss'

/** The matches split by kick-off day in the viewer's zone, keeping their order. */
function byDay(matches: FootballMatch[]): [string, FootballMatch[]][] {
  const days = new Map<string, FootballMatch[]>()
  for (const match of matches) {
    const day = formatMatchDay(match.kickoff)
    days.set(day, [...(days.get(day) ?? []), match])
  }
  return [...days]
}

/** Every Champions League, Premier League, La Liga and Serie A match today and tomorrow, by league and then by day. */
function FootballPage() {
  const state = useFootballMatches()

  return (
    <div className="football-page">
      <Link className="football-back" to="/">
        <ArrowLeft size={16} aria-hidden="true" />
        Dashboard
      </Link>

      <header className="football-header">
        <h1>Football</h1>
        {state.status === 'loaded' && (
          <p className="football-meta">
            Today and tomorrow · Updated{' '}
            <time dateTime={state.data.updatedAt}>{formatKickoffTime(state.data.updatedAt)}</time> ·{' '}
            {state.data.source.name}
          </p>
        )}
      </header>

      {state.status === 'loading' ? (
        <p className="football-message">Loading matches…</p>
      ) : state.status === 'error' ? (
        <p className="football-message football-warning" role="alert">
          Matches are currently unavailable
        </p>
      ) : (
        <div className="football-leagues">
          {state.data.leagues.map((league) => {
            const matches = state.data.matches.filter((match) => match.leagueId === league.id)
            return (
              <section key={league.id} className="football-league" aria-labelledby={`league-${league.id}`}>
                <h2 id={`league-${league.id}`} className="football-league-title">
                  <img className="football-league-logo" src={league.logo} alt="" width={24} height={24} />
                  {league.name}
                  <span className="football-league-country">{league.country}</span>
                </h2>
                {matches.length === 0 ? (
                  <p className="football-league-empty">No matches today or tomorrow</p>
                ) : (
                  byDay(matches).map(([day, dayMatches]) => (
                    <div key={day} className="football-day">
                      <h3 className="football-day-title">{day}</h3>
                      <ul className="football-matches">
                        {dayMatches.map((match) => (
                          <li key={match.id} title={match.venue || undefined}>
                            <MatchRow match={match} />
                          </li>
                        ))}
                      </ul>
                    </div>
                  ))
                )}
              </section>
            )
          })}
        </div>
      )}
    </div>
  )
}

export default FootballPage
