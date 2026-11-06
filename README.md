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

NBRB requests time out after 5 seconds; the API then answers `502` and the frontend shows `—`. The
NBRB currency list is cached for a day and every fetched daily rate for the life of the process, so a
history request also serves later single-day lookups.
