import { type KeyboardEvent, type PointerEvent, useEffect, useMemo, useRef, useState } from 'react'
import type { RatePoint } from '../../api/rates.ts'
import { formatDate, formatDayMonth, formatMonthYear, formatRate, formatYear } from '../../utils/format.ts'
import './RateChart.scss'

const margin = { top: 16, right: 16, bottom: 32, left: 60 }
const dayMs = 86_400_000
const tooltipWidth = 140

interface Tick {
  value: number
  label: string
}

/** Rounds a raw tick step to 1, 2 or 5 times a power of ten. */
function niceStep(raw: number): number {
  const magnitude = 10 ** Math.floor(Math.log10(raw))
  const residual = raw / magnitude
  return (residual > 5 ? 10 : residual > 2 ? 5 : residual > 1 ? 2 : 1) * magnitude
}

function valueTicks(min: number, max: number, count: number): { ticks: Tick[]; low: number; high: number } {
  if (min === max) {
    const pad = Math.abs(min) * 0.01 || 1
    min -= pad
    max += pad
  }

  const step = niceStep((max - min) / count)
  const low = Math.floor(min / step) * step
  const high = Math.ceil(max / step) * step
  const decimals = Math.max(0, -Math.floor(Math.log10(step)))

  const ticks: Tick[] = []
  for (let i = 0; low + i * step <= high + step / 2; i++) {
    const value = low + i * step
    ticks.push({ value, label: value.toFixed(decimals) })
  }

  return { ticks, low, high }
}

/** Calendar-aligned date ticks: days for short spans, month starts for medium, year starts for long. */
function timeTicks(start: number, end: number, maxTicks: number): Tick[] {
  const spanDays = (end - start) / dayMs
  const ticks: Tick[] = []

  if (spanDays <= 90) {
    const step = Math.max(1, Math.ceil(spanDays / Math.max(1, maxTicks - 1))) * dayMs
    for (let t = start; t <= end; t += step) {
      ticks.push({ value: t, label: formatDayMonth(t) })
    }
  } else if (spanDays <= 3 * 365) {
    const step = Math.max(1, Math.ceil(spanDays / 30.44 / maxTicks))
    const first = new Date(start)
    const year = first.getUTCFullYear()
    for (let month = first.getUTCMonth() + 1, t = Date.UTC(year, month, 1); t <= end; month += step) {
      ticks.push({ value: t, label: formatMonthYear(t) })
      t = Date.UTC(year, month + step, 1)
    }
  } else {
    const step = Math.max(1, Math.ceil(spanDays / 365.25 / maxTicks))
    for (let year = new Date(start).getUTCFullYear() + 1, t = Date.UTC(year, 0, 1); t <= end; year += step) {
      ticks.push({ value: t, label: formatYear(t) })
      t = Date.UTC(year + step, 0, 1)
    }
  }

  return ticks
}

interface RateChartProps {
  /** One point per consecutive day, oldest first. */
  points: RatePoint[]
  /** The pair being charted, e.g. "EUR/USD". */
  label: string
}

/** Line chart of a rate over time, with a crosshair tooltip driven by pointer or arrow keys. */
function RateChart({ points, label }: RateChartProps) {
  const containerRef = useRef<HTMLDivElement>(null)
  const [width, setWidth] = useState(0)
  const [active, setActive] = useState<number | null>(null)

  useEffect(() => {
    const container = containerRef.current
    if (!container) return
    const observer = new ResizeObserver(([entry]) => setWidth(entry.contentRect.width))
    observer.observe(container)
    return () => observer.disconnect()
  }, [])

  const height = width < 520 ? 240 : 320

  const chart = useMemo(() => {
    if (points.length === 0 || width === 0) return null

    const plotWidth = Math.max(0, width - margin.left - margin.right)
    const plotHeight = height - margin.top - margin.bottom
    const baseline = margin.top + plotHeight

    const times = points.map((point) => Date.parse(point.date))
    const start = times[0]
    const end = times[times.length - 1]
    const x = (t: number) => margin.left + (end === start ? plotWidth / 2 : ((t - start) / (end - start)) * plotWidth)

    const rates = points.map((point) => point.rate)
    const { ticks: yTicks, low, high } = valueTicks(Math.min(...rates), Math.max(...rates), plotHeight < 200 ? 4 : 5)
    const y = (rate: number) => margin.top + (1 - (rate - low) / (high - low)) * plotHeight

    const line = points.map((point, i) => `${i ? 'L' : 'M'}${x(times[i]).toFixed(1)},${y(point.rate).toFixed(1)}`)
    const linePath = line.join('')
    const areaPath = `${linePath}L${x(end).toFixed(1)},${baseline}L${x(start).toFixed(1)},${baseline}Z`

    return {
      plotWidth,
      baseline,
      x,
      y,
      times,
      linePath,
      areaPath,
      yTicks,
      xTicks: timeTicks(start, end, Math.max(2, Math.floor(plotWidth / 90))),
      low: Math.min(...rates),
      high: Math.max(...rates),
    }
  }, [points, width, height])

  // Points change under a held index when the period changes; drop it rather than point past the end.
  const activeIndex = active !== null && active < points.length ? active : null

  function handlePointerMove(event: PointerEvent<SVGSVGElement>) {
    if (!chart || points.length === 0) return
    const offset = event.clientX - event.currentTarget.getBoundingClientRect().left - margin.left
    // Points are evenly spaced (one per day), so the nearest one is a matter of rounding.
    const index = Math.round((offset / chart.plotWidth) * (points.length - 1))
    setActive(Math.min(points.length - 1, Math.max(0, index)))
  }

  function handleKeyDown(event: KeyboardEvent<SVGSVGElement>) {
    const last = points.length - 1
    const current = activeIndex ?? last
    const next: Record<string, number> = {
      ArrowLeft: current - 1,
      ArrowRight: current + 1,
      Home: 0,
      End: last,
    }
    if (event.key in next) {
      event.preventDefault()
      setActive(Math.min(last, Math.max(0, next[event.key])))
    }
  }

  const activePoint = activeIndex !== null ? points[activeIndex] : null
  const activeX = chart && activeIndex !== null ? chart.x(chart.times[activeIndex]) : 0
  const lastPoint = points[points.length - 1]

  return (
    <div className="rate-chart" ref={containerRef}>
      {chart && (
        <svg
          width={width}
          height={height}
          role="img"
          tabIndex={0}
          aria-label={`${label} from ${formatDate(points[0].date)} to ${formatDate(lastPoint.date)}, ranging from ${formatRate(chart.low)} to ${formatRate(chart.high)}. Use the arrow keys to read individual days.`}
          onPointerMove={handlePointerMove}
          onPointerLeave={() => setActive(null)}
          onKeyDown={handleKeyDown}
          onFocus={() => setActive(points.length - 1)}
          onBlur={() => setActive(null)}
        >
          {chart.yTicks.map((tick) => (
            <g key={tick.value}>
              <line
                className="rate-chart-grid"
                x1={margin.left}
                x2={margin.left + chart.plotWidth}
                y1={chart.y(tick.value)}
                y2={chart.y(tick.value)}
              />
              <text className="rate-chart-axis-label" x={margin.left - 10} y={chart.y(tick.value)} dy="0.32em" textAnchor="end">
                {tick.label}
              </text>
            </g>
          ))}

          {chart.xTicks.map((tick) => (
            <text
              key={tick.value}
              className="rate-chart-axis-label"
              x={chart.x(tick.value)}
              y={chart.baseline + 20}
              textAnchor="middle"
            >
              {tick.label}
            </text>
          ))}

          <path className="rate-chart-area" d={chart.areaPath} />
          <path className="rate-chart-line" d={chart.linePath} />

          {activePoint ? (
            <>
              <line className="rate-chart-crosshair" x1={activeX} x2={activeX} y1={margin.top} y2={chart.baseline} />
              <circle className="rate-chart-dot" cx={activeX} cy={chart.y(activePoint.rate)} r={5} />
            </>
          ) : (
            <circle
              className="rate-chart-dot"
              cx={chart.x(chart.times[chart.times.length - 1])}
              cy={chart.y(lastPoint.rate)}
              r={5}
            />
          )}
        </svg>
      )}

      {chart && activePoint && (
        <div
          className="rate-chart-tooltip"
          style={{
            width: tooltipWidth,
            // Sit beside the crosshair, flipping to its left near the right edge.
            left: activeX + 12 + tooltipWidth > width ? activeX - 12 - tooltipWidth : activeX + 12,
            top: margin.top,
          }}
          aria-hidden="true"
        >
          <span className="rate-chart-tooltip-value">{formatRate(activePoint.rate)}</span>
          <span className="rate-chart-tooltip-date">{formatDate(activePoint.date)}</span>
        </div>
      )}

      <div className="app-visually-hidden" aria-live="polite">
        {activePoint && `${formatDate(activePoint.date)}: ${formatRate(activePoint.rate)}`}
      </div>
    </div>
  )
}

export default RateChart
