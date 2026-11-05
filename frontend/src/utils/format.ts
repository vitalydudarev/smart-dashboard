// Dates from the backend are date-only strings, which parse as UTC midnight. Formatting them in UTC
// keeps them from showing as the previous day west of Greenwich.

const fullDate = new Intl.DateTimeFormat('en-GB', { dateStyle: 'medium', timeZone: 'UTC' })
const dayMonth = new Intl.DateTimeFormat('en-GB', { day: 'numeric', month: 'short', timeZone: 'UTC' })
const monthYear = new Intl.DateTimeFormat('en-GB', { month: 'short', year: 'numeric', timeZone: 'UTC' })
const year = new Intl.DateTimeFormat('en-GB', { year: 'numeric', timeZone: 'UTC' })

export const formatDate = (date: string | number) => fullDate.format(new Date(date))
export const formatDayMonth = (date: number) => dayMonth.format(date)
export const formatMonthYear = (date: number) => monthYear.format(date)
export const formatYear = (date: number) => year.format(date)

// Five significant digits reads naturally across magnitudes: 1.0842, 163.27, 0.0066716.
const rate = new Intl.NumberFormat('en-GB', { minimumSignificantDigits: 5, maximumSignificantDigits: 5 })
const rateChange = new Intl.NumberFormat('en-GB', { maximumSignificantDigits: 3, signDisplay: 'exceptZero' })
const percentChange = new Intl.NumberFormat('en-GB', {
  style: 'percent',
  maximumFractionDigits: 2,
  signDisplay: 'exceptZero',
})

export const formatRate = (value: number) => rate.format(value)
export const formatRateChange = (value: number) => rateChange.format(value)
export const formatPercentChange = (ratio: number) => percentChange.format(ratio)

/** Today's date in the viewer's time zone, as YYYY-MM-DD. */
export function todayIso(): string {
  const now = new Date()
  const pad = (n: number) => String(n).padStart(2, '0')
  return `${now.getFullYear()}-${pad(now.getMonth() + 1)}-${pad(now.getDate())}`
}
