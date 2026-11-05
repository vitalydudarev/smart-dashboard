import { ArrowDownRight, ArrowLeft, ArrowLeftRight, ArrowUpRight, Minus } from 'lucide-react'
import { useEffect, useState } from 'react'
import { Link, useSearchParams } from 'react-router'
import {
  type Currency,
  type RangePreset,
  type RateHistory,
  type RatePeriod,
  fetchCurrencies,
  fetchRateHistory,
} from '../../api/rates.ts'
import RateChart from '../../components/RateChart/RateChart.tsx'
import { formatDate, formatPercentChange, formatRate, formatRateChange, todayIso } from '../../utils/format.ts'
import './RatesPage.scss'

type RangeOption = RangePreset | 'custom'

const rangeOptions: readonly { value: RangeOption; label: string; title: string }[] = [
  { value: '7d', label: '7D', title: 'Last 7 days' },
  { value: '30d', label: '30D', title: 'Last 30 days' },
  { value: '6m', label: '6M', title: 'Last 6 months' },
  { value: '1y', label: '1Y', title: 'Last year' },
  { value: '5y', label: '5Y', title: 'Last 5 years' },
  { value: 'custom', label: 'Custom', title: 'Custom range' },
]

const defaultRange: RangeOption = '30d'

function isRangeOption(value: string | null): value is RangeOption {
  return rangeOptions.some((option) => option.value === value)
}

/** The selected pair and period live in the URL, so a view can be bookmarked or shared. */
function useRateSelection() {
  const [params, setParams] = useSearchParams()
  const rangeParam = params.get('range')

  const selection = {
    base: params.get('base') ?? 'EUR',
    quote: params.get('quote') ?? 'USD',
    range: isRangeOption(rangeParam) ? rangeParam : defaultRange,
    from: params.get('from') ?? '',
    to: params.get('to') ?? '',
  }

  function update(changes: Partial<Record<keyof typeof selection, string | null>>) {
    // Start from the live URL: the router's updater argument is the params from the last render, so
    // two changes before a re-render would otherwise drop the first.
    const next = new URLSearchParams(window.location.search)
    for (const [key, value] of Object.entries(changes)) {
      if (value) next.set(key, value)
      else next.delete(key)
    }
    setParams(next, { replace: true })
  }

  return [selection, update] as const
}

function RatesPage() {
  const [selection, update] = useRateSelection()
  const { base, quote, range, from, to } = selection
  const pair = `${base}/${quote}`

  const [currencies, setCurrencies] = useState<Currency[]>([])
  // Tagged with the request it answers, so a newer selection reads as loading until its own reply.
  const [result, setResult] = useState<{ key: string; history: RateHistory | null; error: string | null }>({
    key: '',
    history: null,
    error: null,
  })

  useEffect(() => {
    const controller = new AbortController()
    fetchCurrencies(controller.signal)
      .then(setCurrencies)
      .catch((error: unknown) => {
        if (!controller.signal.aborted) console.error(error)
      })
    return () => controller.abort()
  }, [])

  const customInvalid = range === 'custom' && (!from || !to || from > to)
  const requestKey = [base, quote, range, from, to].join('|')
  const loading = !customInvalid && result.key !== requestKey
  const history = result.history
  const error = result.key === requestKey ? result.error : null

  useEffect(() => {
    if (customInvalid) return

    const period: RatePeriod = range === 'custom' ? { from, to } : { range }
    const controller = new AbortController()

    // The previous chart stays up, dimmed, until the new data arrives.
    fetchRateHistory(base, quote, period, controller.signal)
      .then((history) => setResult({ key: requestKey, history, error: null }))
      .catch((error: unknown) => {
        if (controller.signal.aborted) return
        const message = error instanceof Error ? error.message : 'Could not load rates'
        setResult((previous) => ({ key: requestKey, history: previous.history, error: message }))
      })

    return () => controller.abort()
  }, [base, quote, range, from, to, customInvalid, requestKey])

  function selectCurrency(side: 'base' | 'quote', code: string) {
    const other = side === 'base' ? quote : base
    // Picking the currency already on the other side swaps the pair instead of making EUR/EUR.
    if (code === other) update({ base: quote, quote: base })
    else update({ [side]: code })
  }

  function selectRange(value: RangeOption) {
    if (value === 'custom') {
      // Start the custom range from whatever period is on screen.
      update({ range: value, from: from || history?.from, to: to || history?.to })
    } else {
      update({ range: value === defaultRange ? null : value, from: null, to: null })
    }
  }

  const points = history?.points ?? []
  const first = points[0]
  const last = points[points.length - 1]
  const change = first && last ? last.rate - first.rate : 0
  const ChangeIcon = change > 0 ? ArrowUpRight : change < 0 ? ArrowDownRight : Minus
  const rates = points.map((point) => point.rate)

  const currencyOptions = currencies.length
    ? currencies
    : [base, quote].map((code) => ({ code, name: code }))

  return (
    <div className="rates-page">
      <Link className="rates-back" to="/">
        <ArrowLeft size={16} aria-hidden="true" />
        Dashboard
      </Link>

      <header className="rates-header">
        <h1>Exchange rates</h1>

        <div className="rates-pair">
          <label className="rates-select">
            <span className="app-visually-hidden">Base currency</span>
            <select value={base} onChange={(event) => selectCurrency('base', event.target.value)}>
              {currencyOptions.map((currency) => (
                <option key={currency.code} value={currency.code}>
                  {currency.code} · {currency.name}
                </option>
              ))}
            </select>
          </label>

          <button
            type="button"
            className="rates-swap"
            onClick={() => update({ base: quote, quote: base })}
            aria-label={`Swap to ${quote}/${base}`}
            title="Swap currencies"
          >
            <ArrowLeftRight size={18} />
          </button>

          <label className="rates-select">
            <span className="app-visually-hidden">Quote currency</span>
            <select value={quote} onChange={(event) => selectCurrency('quote', event.target.value)}>
              {currencyOptions.map((currency) => (
                <option key={currency.code} value={currency.code}>
                  {currency.code} · {currency.name}
                </option>
              ))}
            </select>
          </label>
        </div>
      </header>

      <div className="rates-filters">
        <div className="segmented rates-range" role="group" aria-label="Time range">
          {rangeOptions.map(({ value, label, title }) => (
            <button
              key={value}
              type="button"
              className={value === range ? 'segment segment--selected' : 'segment'}
              aria-pressed={value === range}
              title={title}
              onClick={() => selectRange(value)}
            >
              {label}
            </button>
          ))}
        </div>

        {range === 'custom' && (
          <div className="rates-custom">
            <label>
              <span>From</span>
              <input
                type="date"
                value={from}
                max={to || todayIso()}
                onChange={(event) => update({ from: event.target.value })}
              />
            </label>
            <label>
              <span>To</span>
              <input
                type="date"
                value={to}
                min={from}
                max={todayIso()}
                onChange={(event) => update({ to: event.target.value })}
              />
            </label>
            {customInvalid && from && to && <span className="rates-hint">The start date must be before the end date.</span>}
          </div>
        )}
      </div>

      <section className="rates-card" aria-busy={loading} aria-label={`${pair} rate history`}>
        {error ? (
          <p className="rates-message" role="alert">
            {error}
          </p>
        ) : !history ? (
          <p className="rates-message">Loading…</p>
        ) : (
          <div className={loading ? 'rates-body rates-body--stale' : 'rates-body'}>
            <div className="rates-summary">
              <div>
                <div className="rates-summary-pair">{pair}</div>
                <div className="rates-summary-rate">{last ? formatRate(last.rate) : '—'}</div>
                {last && (
                  <time className="rates-summary-date" dateTime={last.date}>
                    {formatDate(last.date)}
                  </time>
                )}
              </div>

              {first && last && (
                <dl className="rates-stats">
                  <div>
                    <dt>Change</dt>
                    <dd className="rates-change">
                      <ChangeIcon size={16} aria-hidden="true" />
                      {formatRateChange(change)} ({formatPercentChange(change / first.rate)})
                    </dd>
                  </div>
                  <div>
                    <dt>High</dt>
                    <dd>{formatRate(Math.max(...rates))}</dd>
                  </div>
                  <div>
                    <dt>Low</dt>
                    <dd>{formatRate(Math.min(...rates))}</dd>
                  </div>
                </dl>
              )}
            </div>

            <RateChart points={points} label={pair} />

            <p className="rates-period">
              {formatDate(history.from)} – {formatDate(history.to)} · {points.length} days
            </p>

            <details className="rates-table">
              <summary>Show data table</summary>
              <div className="rates-table-scroll">
                <table>
                  <thead>
                    <tr>
                      <th scope="col">Date</th>
                      <th scope="col">{pair}</th>
                    </tr>
                  </thead>
                  <tbody>
                    {points.toReversed().map((point) => (
                      <tr key={point.date}>
                        <td>{formatDate(point.date)}</td>
                        <td>{formatRate(point.rate)}</td>
                      </tr>
                    ))}
                  </tbody>
                </table>
              </div>
            </details>
          </div>
        )}
      </section>
    </div>
  )
}

export default RatesPage
