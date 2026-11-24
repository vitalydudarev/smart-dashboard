import type { FootballMatch, FootballSide } from '../api/football.ts'
import { formatKickoffTime, formatMatchClock, formatMatchDay, matchPhase } from './useFootballMatches.ts'
import './MatchRow.scss'

function Team({ side }: { side: FootballSide }) {
  return (
    <span className="match-team">
      <img className="match-team-logo" src={side.logo} alt="" width={20} height={20} loading="lazy" />
      <span className="match-team-name">{side.name}</span>
    </span>
  )
}

/** What the right-hand column says: the score once the match has started, else the kick-off. */
function MatchState({ match, showDay }: { match: FootballMatch; showDay: boolean }) {
  const phase = matchPhase(match)
  const score =
    match.home.goals != null && match.away.goals != null ? `${match.home.goals} – ${match.away.goals}` : null

  if (phase === 'live' || phase === 'finished') {
    return (
      <span className="match-state">
        <span className="match-score">
          <span className="app-visually-hidden">Score </span>
          {score ?? '–'}
        </span>
        {phase === 'live' ? (
          <span className="match-badge match-badge--live">
            <span className="match-live-dot" aria-hidden="true" />
            {formatMatchClock(match)}
          </span>
        ) : (
          <span className="match-badge">{match.status.short}</span>
        )}
      </span>
    )
  }
  if (phase === 'other') {
    return (
      <span className="match-state">
        <span className="match-badge match-badge--warning" title={match.status.long}>
          {match.status.long}
        </span>
      </span>
    )
  }
  return (
    <span className="match-state">
      <time className="match-kickoff" dateTime={match.kickoff}>
        {formatKickoffTime(match.kickoff)}
      </time>
      {showDay && <span className="match-day">{formatMatchDay(match.kickoff)}</span>}
    </span>
  )
}

/** One match: both teams stacked on the left, the score or kick-off on the right. */
function MatchRow({ match, showDay = false }: { match: FootballMatch; showDay?: boolean }) {
  const live = matchPhase(match) === 'live'
  return (
    <div
      className={['match-row', match.featured && 'match-row--featured', live && 'match-row--live']
        .filter(Boolean)
        .join(' ')}
    >
      <span className="match-teams">
        <Team side={match.home} />
        <Team side={match.away} />
      </span>
      <MatchState match={match} showDay={showDay} />
    </div>
  )
}

export default MatchRow
