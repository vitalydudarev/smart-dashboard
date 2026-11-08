# smart-dashboard

Monorepo for the app.

- [`frontend/`](frontend) – React app
- [`backend/`](backend) – Go API server

## Running locally

Start the backend (listens on `:8080`, override with `PORT`):

```sh
cd backend && go run .
```

Start the frontend dev server; it proxies `/api` to the backend:

```sh
cd frontend && npm run dev
```

## API

Pairs with BYN come from the official rates of the
[National Bank of the Republic of Belarus](https://api.nbrb.by/exrates) (from 2016-07-01, when BYN
replaced BYR). Every other pair is mocked: each currency follows a deterministic random walk from
2016-01-01 to the day the backend started. Responses carry a `source` saying which one answered.

- `GET /api/currencies` – supported currencies
- `GET /api/rates/latest?base=EUR&quote=USD` – the latest rate for a pair; add `date=YYYY-MM-DD`
  for the rate on a given day
- `GET /api/rates/history?base=EUR&quote=USD&range=30d` – daily rates for a preset range
  (`7d`, `30d`, `6m`, `1y`, `5y`), or with `from=YYYY-MM-DD&to=YYYY-MM-DD` instead of `range`
- `GET /api/weather?lat=53.9&lng=27.56` – the latest temperature and humidity from the
  [Belhydromet](https://pogoda.by) station nearest the coordinates (Minsk when they are left out)
- `GET /api/weather/stations?lat=53.9&lng=27.56` – every station with its distance, nearest first
- `GET /api/weather/stations/26850?lat=53.9&lng=27.56` – the latest observation at one station
  (`404` when the station is unknown or has no current observation)
- `GET /api/weather/stations/26850/forecast` – one station's forecast by day: the lowest and highest
  temperature and the midday weather (`404` when the station is unknown or has no forecast)
- `GET /api/weather/radar` – the URL of the Minsk radar's (`UMMN`) latest map on pogoda.by
- `GET /api/weather/satellite` – the URL of the latest satellite image of channel `012` on pogoda.by
  (both `404` when the list has no such image)

NBRB requests time out after 5 seconds; the API then answers `502` and the frontend shows `—`. The
NBRB currency list is cached for a day and every fetched daily rate for the life of the process, so a
history request also serves later single-day lookups.

Weather comes from Belhydromet's station list (cached for a day) and the latest observation of the
nearest station. When a station fails or reports no temperature, the next nearest is tried, up to five;
if none answers the API returns `502` and the widget shows `—` with a warning. The widget asks the
browser for the viewer's location (falling back to Minsk) and refreshes every 5 minutes. Clicking it
opens `/weather`, which lists every station nearest first, with a search box; each row loads its
weather when it scrolls into view. Observations are cached for 5 minutes. A panel beside the list shows the
forecast for the selected row (the first one until another is clicked); forecasts are never cached. Below the forecast are the latest radar map
and satellite image; clicking one opens it full size. Their image lists are never cached either.
