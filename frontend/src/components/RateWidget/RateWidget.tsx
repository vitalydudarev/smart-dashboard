import { mockRate } from './mockRate.ts'
import './RateWidget.scss'

// UTC because a date-only string parses as UTC midnight, which is the previous day west of Greenwich.
const dateFormat = new Intl.DateTimeFormat('en-GB', { dateStyle: 'medium', timeZone: 'UTC' })

function RateWidget() {
  const { pair, rate, date } = mockRate

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
