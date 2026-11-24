import { ChevronRight, Trophy } from 'lucide-react'
import { Link } from 'react-router'
import MatchRow from '../../football/MatchRow.tsx'
import { formatKickoffTime, matchPhase, useFootballMatches } from '../../football/useFootballMatches.ts'
import './FootballWidget.scss'

/** How many of the featured clubs' matches the card lists. */
const SHOWN = 3

/**
 * The featured clubs' matches that are in progress or still to come today and tomorrow, live ones first;
 * opens the page with every match of their leagues.
 */
function FootballWidget() {
  const state = useFootballMatches()

  const top =
    state.status === 'loaded'
      ? state.data.matches
          .filter((match) => match.featured)
          .filter((match) => ['live', 'upcoming'].includes(matchPhase(match)))
          // Stable, so each group stays in kick-off order.
          .sort((a, b) => Number(matchPhase(b) === 'live') - Number(matchPhase(a) === 'live'))
      : []

  return (
    <Link
      className="football-widget"
      to="/football"
      aria-label="Top football matches. View every Premier League, La Liga and Serie A match"
      aria-busy={state.status === 'loading'}
    >
      <span className="football-widget-title">
        <Trophy className="football-widget-icon" size={16} aria-hidden="true" />
        <span className="football-widget-title-text">Top matches</span>
        <ChevronRight className="football-widget-chevron" size={18} aria-hidden="true" />
      </span>

      {state.status === 'loading' ? (
        <span className="football-widget-message">Loading…</span>
      ) : state.status === 'error' ? (
        <span className="football-widget-message football-widget-warning" role="status">
          Matches are currently unavailable
        </span>
      ) : top.length === 0 ? (
        <span className="football-widget-message">No matches for your clubs today or tomorrow</span>
      ) : (
        <ul className="football-widget-matches">
          {top.slice(0, SHOWN).map((match) => (
            <li key={match.id}>
              <MatchRow match={match} showDay />
            </li>
          ))}
        </ul>
      )}

      {state.status === 'loaded' && (
        <span className="football-widget-footer">
          {top.length > SHOWN && <span>+{top.length - SHOWN} more · </span>}
          <time dateTime={state.data.updatedAt}>Updated {formatKickoffTime(state.data.updatedAt)}</time>
        </span>
      )}
    </Link>
  )
}

export default FootballWidget
