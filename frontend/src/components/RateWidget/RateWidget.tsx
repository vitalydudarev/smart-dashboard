import { ChevronRight } from 'lucide-react'
import { useEffect, useState } from 'react'
import { Link } from 'react-router'
import { type LatestRate, fetchLatestRate } from '../../api/rates.ts'
import { formatDate, formatRate } from '../../utils/format.ts'
import './RateWidget.scss'

const base = 'USD'
const quote = 'BYN'
const pair = `${base}/${quote}`

type RateState = { status: 'loading' } | { status: 'error' } | { status: 'loaded'; rate: LatestRate }

/** The headline USD/BYN rate (today's, or yesterday's until today's is published); opens the rates page for that pair. */
function RateWidget() {
  const [state, setState] = useState<RateState>({ status: 'loading' })

  useEffect(() => {
    const controller = new AbortController()
    fetchLatestRate(base, quote, controller.signal)
      .then((rate) => setState({ status: 'loaded', rate }))
      .catch((error: unknown) => {
        if (!controller.signal.aborted) {
          console.error(error)
          setState({ status: 'error' })
        }
      })
    return () => controller.abort()
  }, [])

  return (
    <Link
      className="rate-widget"
      to={`/rates?base=${base}&quote=${quote}`}
      aria-label={`${pair} exchange rate. View history and other currencies`}
      aria-busy={state.status === 'loading'}
    >
      <span className="rate-widget-pair">
        {pair}
        <ChevronRight className="rate-widget-chevron" size={18} aria-hidden="true" />
      </span>
      {state.status === 'loaded' ? (
        <>
          <span className="rate-widget-rate">{formatRate(state.rate.rate)}</span>
          <time className="rate-widget-date" dateTime={state.rate.date}>
            {formatDate(state.rate.date)}
          </time>
        </>
      ) : (
        <>
          <span className="rate-widget-rate">—</span>
          <span className="rate-widget-date">{state.status === 'loading' ? 'Loading…' : 'Rate unavailable'}</span>
        </>
      )}
    </Link>
  )
}

export default RateWidget
