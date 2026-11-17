import { useEffect, useState } from 'react'
import { Link } from 'react-router'
import { type LatestRate, fetchLatestRate } from '../../api/rates.ts'
import { formatRate, formatTemperature } from '../../utils/format.ts'
import { useShownWeather } from '../../weather/useShownWeather.ts'

type RateState = { status: 'loading' } | { status: 'error' } | { status: 'loaded'; rate: LatestRate }

/** The latest USD/BYN rate, fetched once per page load. */
function useUsdRate(): RateState {
  const [state, setState] = useState<RateState>({ status: 'loading' })

  useEffect(() => {
    const controller = new AbortController()
    fetchLatestRate('USD', 'BYN', controller.signal)
      .then((rate) => setState({ status: 'loaded', rate }))
      .catch((error: unknown) => {
        if (!controller.signal.aborted) {
          console.error(error)
          setState({ status: 'error' })
        }
      })
    return () => controller.abort()
  }, [])

  return state
}

/**
 * The USD/BYN rate and the temperature at the starred (or nearest) station, side by side in the navbar;
 * each links to its page. A value that has not loaded, or failed to, shows as a dash.
 */
function NavbarStats() {
  const rate = useUsdRate()
  const weather = useShownWeather()

  return (
    <>
      <Link
        className="header-stat"
        to="/rates?base=USD&quote=BYN"
        title="USD/BYN exchange rate"
        aria-busy={rate.status === 'loading'}
      >
        <span className="app-visually-hidden">USD/BYN </span>
        {rate.status === 'loaded' ? `$${formatRate(rate.rate.rate)}` : '$—'}
      </Link>
      <Link
        className="header-stat"
        to="/weather"
        title={weather.status === 'loaded' ? `Temperature in ${weather.weather.location.name}` : 'Temperature'}
        aria-busy={weather.status === 'loading'}
      >
        <span className="app-visually-hidden">
          {weather.status === 'loaded' ? `Temperature in ${weather.weather.location.name} ` : 'Temperature '}
        </span>
        {weather.status === 'loaded' ? formatTemperature(weather.weather.temperature) : '—'}
      </Link>
    </>
  )
}

export default NavbarStats
