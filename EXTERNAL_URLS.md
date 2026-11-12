# External URLs

Every external URL the app talks to. The backend (`backend/`) calls the upstream APIs; the
frontend only reaches out directly for map tiles and the static weather map image.

## National Bank of the Republic of Belarus (exchange rates)

Base URL: `https://api.nbrb.by/exrates` — `nbrbBaseURL` in [backend/nbrb.go](backend/nbrb.go)

| Upstream URL | Query parameters | Client method | Used by |
| --- | --- | --- | --- |
| `https://api.nbrb.by/exrates/currencies` | — | `currencies` | `GET /api/currencies`, currency lookups |
| `https://api.nbrb.by/exrates/rates/{id}` | `parammode`, `ondate` | `rate` | `GET /api/rates/latest` |
| `https://api.nbrb.by/exrates/rates/dynamics/{id}` | `startdate`, `enddate` | `dynamics` | `GET /api/rates/history` |
| `https://api.nbrb.by/exrates/currencies/{id}` | — | `currency` | not called (defined only) |
| `https://api.nbrb.by/exrates/rates` | `ondate`, `periodicity` | `rates` | not called (defined only) |

## Belhydromet — pogoda.by (weather)

Base URL: `https://pogoda.by/api/v2` — `pogodaBaseURL` in [backend/pogoda.go](backend/pogoda.go)

| Upstream URL | Query parameters | Client method | Used by |
| --- | --- | --- | --- |
| `https://pogoda.by/api/v2/weather-fact/all-cities` | — | `cities` | station list, `GET /api/weather/stations` |
| `https://pogoda.by/api/v2/weather-fact` | `station` | `fact` | `GET /api/weather`, `GET /api/weather/stations/{id}` |
| `https://pogoda.by/api/v2/maps/meteo-10min` | — | `meteo10min` | `GET /api/weather/map` |
| `https://pogoda.by/api/v2/numeric-weather/12/{station}` | — | `forecast` | `GET /api/weather/stations/{id}/forecast` |
| `https://pogoda.by/api/v2/radar/last` | — | `radars` | `GET /api/weather/radar` |
| `https://pogoda.by/api/v2/satellite` | — | `satellites` | `GET /api/weather/satellite` |

### Image files

Base URL: `https://pogoda.by/files` — `pogodaFilesURL` in [backend/pogoda.go](backend/pogoda.go).
The backend does not fetch these; it returns the URLs and the browser loads them.

| URL | Returned by |
| --- | --- |
| `https://pogoda.by/files/radars/static/{locationId}/{fileName}` (radar `UMMN`, Minsk) | `GET /api/weather/radar` |
| `https://pogoda.by/files/satellite/{fileName}` (channel `012`, `.jpg`) | `GET /api/weather/satellite` |

## Frontend (loaded directly by the browser)

| URL | Purpose | Where |
| --- | --- | --- |
| `https://pogoda.by/files/maps/static/fact/RB_TNOW.png` | Static current-weather map of Belarus | [frontend/src/api/weather.ts](frontend/src/api/weather.ts) |
| `https://tile.openstreetmap.org/{z}/{x}/{y}.png` | Leaflet map tiles | [frontend/src/pages/WeatherPage/WeatherMap.tsx](frontend/src/pages/WeatherPage/WeatherMap.tsx) |
| `https://www.openstreetmap.org/copyright` | Map attribution link | [frontend/src/pages/WeatherPage/WeatherMap.tsx](frontend/src/pages/WeatherPage/WeatherMap.tsx) |

## Local development

| URL | Purpose |
| --- | --- |
| `http://localhost:8080` | Backend, proxied from Vite for `/api` ([frontend/vite.config.ts](frontend/vite.config.ts)) |
