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
