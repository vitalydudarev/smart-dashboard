import { useEffect, useState } from 'react'
import { type ExchangeRate, fetchEurUsdRate } from './rateApi.ts'
import './RateWidget.scss'

// UTC because a date-only string parses as UTC midnight, which is the previous day west of Greenwich.
const dateFormat = new Intl.DateTimeFormat('en-GB', { dateStyle: 'medium', timeZone: 'UTC' })

type RateState =
  | { status: 'loading' }
  | { status: 'error' }
  | { status: 'loaded'; rate: ExchangeRate }

function RateWidget() {
  const [state, setState] = useState<RateState>({ status: 'loading' })

  useEffect(() => {
    const controller = new AbortController()
    fetchEurUsdRate(controller.signal)
      .then((rate) => setState({ status: 'loaded', rate }))
      .catch((error: unknown) => {
        if (!controller.signal.aborted) {
          console.error(error)
          setState({ status: 'error' })
        }
      })
    return () => controller.abort()
  }, [])

  if (state.status !== 'loaded') {
    return (
      <section className="rate-widget" aria-label="EUR/USD exchange rate" aria-busy={state.status === 'loading'}>
        <span className="rate-widget-pair">EUR/USD</span>
        <span className="rate-widget-rate">—</span>
        <span className="rate-widget-date">
          {state.status === 'loading' ? 'Loading…' : 'Rate unavailable'}
        </span>
      </section>
    )
  }

  const { pair, rate, date } = state.rate

  return (
    <section className="rate-widget" aria-label={`${pair} exchange rate`}>
      <span className="rate-widget-pair">{pair}</span>
      <span className="rate-widget-rate">{rate.toFixed(4)}</span>
      <time className="rate-widget-date" dateTime={date}>
        {dateFormat.format(new Date(date))}
      </time>
    </section>
  )
}

export default RateWidget
