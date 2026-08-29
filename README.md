# go-dashboarder

*Čeština: [README.cz.md](README.cz.md)*

A small home-IoT/telemetry dashboard, written in Go from scratch as a
personal/learning project - not a synthetic toy, it runs against a real
setup (real sensors, real weather data). It's a rewrite of an earlier
version, done for better visibility and readability. It ingests sensor and
weather data over MQTT, normalizes it, stores it in TimescaleDB, and serves
a live dashboard over HTTP.

## Architecture

Everything communicates over a single internal MQTT broker (`mqtt_broker`),
which acts as the event bus between independently deployable services. Data
flows through three stages, each publishing to a topic the next stage
subscribes to:

```
[ingesters]  --input/#-->  [parsers]  --/data/{iot,web}/#-->  [storager]  -->  TimescaleDB
raw-input                  iot-parser                                          |
mqtt-inputer                web-parser                                         v
openmeteo-input                                                          api (HTTP) --> frontend + Caddy
```

- **Ingesters** (`cmd/raw-input`, `cmd/mqtt-inputer`, `cmd/openmeteo-input`,
  `cmd/owm-input`) get data into the system from the outside world (a raw TCP
  socket, a bridged external MQTT broker, or a weather API) and publish it,
  wrapped in a signed envelope, to `input/...`.
- **Parsers** (`cmd/iot-parser`, `cmd/web-parser`) subscribe to `input/#`,
  normalize the payload into a canonical shape, and republish it to
  `/data/iot/...` or `/data/web/...`.
- **`cmd/storager`** subscribes to `/data/iot/#` and `/data/web/#` and
  persists everything into TimescaleDB.
- **`cmd/api`** serves the persisted data and per-component health status as
  JSON, and also listens on `status/#` to track which components are alive.
- **`cmd/frontend`** renders the HTML dashboard (Chart.js) that calls the API
  from the browser.
- **Caddy** sits in front of `frontend` and `api` as a single reverse-proxy
  entry point (port 3800).

Every long-running service also publishes to `status/<client-id>` on
connect (retained "alive") and via a periodic heartbeat, with "offline" as
the MQTT Last Will, so `api` can track liveness without polling.

See `architektura/dashoarderHL.png` / `.drawio` for the original hand-drawn
diagram this evolved from.

## Services (`cmd/`)

| Service | Role |
|---|---|
| `raw-input` | TCP socket ingester for legacy `metric:value` sensor feeds |
| `mqtt-inputer` | Bridges an external MQTT broker/topic into the internal bus |
| `openmeteo-input` | Polls the Open-Meteo API on a timer, publishes as IoT data |
| `owm-input` | Same idea, for OpenWeatherMap (currently unused in `docker-compose.yaml`) |
| `iot-parser` | Normalizes raw IoT payloads (`metric:value` or topic-derived) |
| `web-parser` | Normalizes raw scraped/web payloads |
| `storager` | Subscribes to normalized data, writes to TimescaleDB |
| `api` | HTTP API: latest events, component health |
| `frontend` | Renders the dashboard HTML/JS |

## Design patterns used, and why

- **Factory** — `internal/storage/factory.go` (`NewStorage`) picks a
  `Storage` implementation (`timescaledb` or `mock`) based on config, so the
  rest of the code only ever depends on the `Storage` interface, not a
  concrete database driver. This is what makes `mock` usable for local
  development without a real Postgres instance.
- **Strategy / interface segregation** — `internal/storage/interface.go`
  defines `Storage` as the single contract both `TimescaleStorage` and
  `MockStorage` satisfy. Callers (`storager`, `api`) are written against the
  interface and never know which one they got.
- **Generics as a mini-framework** — `internal/config.LoadConfig[T any]`
  loads a YAML file into any config struct type `T`, then overlays
  environment variables on top via struct tags (`caarlos0/env`). Every
  `cmd/*` service defines its own config struct (e.g. `MqttIngesterConfig`,
  `OpenWeatherConfig`) that embeds the shared `BaseConfig`, so common fields
  (log level, MQTT, secret key) don't get redefined per service, while each
  service still gets its own strongly-typed extra fields.
- **Adapter** — `internal/platform/mqtt/client.go` wraps the raw
  `paho.mqtt.golang` client in a small `Client` type that adds
  application-specific behaviour (status topic, heartbeat, LWT) the
  underlying library doesn't know about, so the rest of the codebase never
  touches `paho` directly.
- **Pipes and filters** — the ingester → parser → storager chain (see
  Architecture above) is a classic pipeline: each stage is a separate OS
  process with one job, consuming one topic and producing another. Stages
  can be restarted, redeployed, or replaced independently.
- **Publish/subscribe as the integration style** — services never call each
  other directly; the MQTT broker is the only thing every service depends
  on. This is what lets `docker-compose.yaml` bring services up in almost
  any order (aside from the broker/DB health-check gates) without them
  needing to know about each other's addresses.
- **Heartbeat / dead man's switch** — `Client.StartHeartbeat` plus the MQTT
  Last Will (`SetWill(..., "dead", ...)`) means `api`'s `HealthManager` finds
  out a component died even if it crashes ungracefully, without needing an
  explicit shutdown handler to fire.

## Running it

```sh
cp .env.example .env   # fill in real values
docker compose up -d --build
```

This brings up an internal Mosquitto broker, a TimescaleDB instance (schema
from `init.sql`), a Valkey instance (provisioned, not yet used by any
service), all the app services above, and Caddy on `:3800`.

Ports (see `.env.example` to change): frontend+api via Caddy on `3800`,
`api` directly on `8080`, `frontend` directly on `8180`, Postgres on `5433`
(kept off the default `5432` since that's commonly already in use),
Mosquitto on `1884`, Valkey on `6380`.

`mqtt-inputer` additionally bridges in an external DS18B20 sensor feed
(`/msh/internal_temp/ds1`/`ds2`) from a different MQTT broker reachable via
`host.docker.internal` — see `SOURCE_MQTT_URL`/`SOURCE_MQTT_TOPIC` below if
you don't have that source and want to disable it.

### Configuration (`.env`)

`.env` is where all secrets and per-environment values live; it's gitignored
and must never be committed. `.env.example` is the checked-in, secret-free
template — copy it to `.env` and fill in real values. Anything not
overridden by an actual environment variable falls back to what's in
`.env.example`/`docker-compose.yaml`.

| Variable | Used by | Meaning |
|---|---|---|
| `MQTT_PORT` | `mqtt_broker` (host port) | Host-side port for the internal Mosquitto broker (container always listens on `1883`) |
| `MQTT_BROKER_URL` | all app services | Internal broker address app services connect to, e.g. `tcp://mqtt_broker:1883` |
| `DB_USER` / `DB_PASSWORD` / `DB_NAME` | `timescale`, `api`, `storager` | Postgres role/database. Only takes effect on TimescaleDB's *first* boot against an empty data directory - changing it later requires a fresh `/srv/go-dashboarder/pgsql-data` |
| `VALKEY_PORT` / `VALKEY_PASSWORD` | `valkey` (host port) | Host-side port and auth password for Valkey (provisioned, not yet used by any service) |
| `SOURCE_MQTT_URL` | `mqtt-inputer` | Address of the *external* broker to bridge from (not the internal one) |
| `SOURCE_MQTT_TOPIC` | `mqtt-inputer` | Topic (wildcards ok, e.g. `/msh/internal_temp/#`) to subscribe to on that external broker |
| `SECRET_KEY` | all app services | Shared HMAC key for signing/verifying messages between ingesters and parsers - must be identical everywhere |
| `LOG_LEVEL` | most services | Default log verbosity (`debug`/`info`/`warn`/...) |

### Building standalone binaries (e.g. for FreeBSD)

`build.sh` cross-compiles a statically linked binary for every `cmd/`
component and drops it, together with `configs/` and `web/`, into
`_release/<GOOS>-<GOARCH>/`:

```sh
./build.sh                          # defaults to freebsd/amd64
GOOS=freebsd GOARCH=arm64 ./build.sh
GOOS=linux   GOARCH=arm64 ./build.sh
```

It's written in POSIX `/bin/sh` (no bashisms), since FreeBSD doesn't ship
bash by default. Copy the resulting `_release/<target>/` directory to the
target machine and run the binaries from inside it (they expect `configs/`
and `web/` as siblings, same as when built into the Docker images).

## Known rough edges

- `cmd/owm-input` (OpenWeatherMap) is superseded by `cmd/openmeteo-input`
  (Open-Meteo, no API key needed) and isn't wired into `docker-compose.yaml`.
- Valkey is provisioned in `docker-compose.yaml` but no service uses it yet.
- `postgresql-old.sql` is kept around as historical reference for the schema
  this evolved from; `init.sql` is the one actually applied.
