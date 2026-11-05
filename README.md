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

All rates are mocked: each currency follows a deterministic random walk from 2016-01-01 to the day
the backend started.

- `GET /api/currencies` – supported currencies
- `GET /api/rates/latest?base=EUR&quote=USD` – the latest rate for a pair
- `GET /api/rates/history?base=EUR&quote=USD&range=30d` – daily rates for a preset range
  (`7d`, `30d`, `6m`, `1y`, `5y`), or with `from=YYYY-MM-DD&to=YYYY-MM-DD` instead of `range`
