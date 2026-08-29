# go-dashboarder

*English: [README.md](README.md)*

Malý domácí IoT/telemetrický dashboard napsaný v Go od nuly jako osobní
výukový projekt - ne syntetická hračka, běží nad reálným setupem (skutečné
senzory, skutečná data o počasí). Je to přepis dřívější verze, kvůli lepší
přehlednosti a čitelnosti. Sbírá data ze senzorů a počasí přes MQTT,
normalizuje je, ukládá do TimescaleDB a servíruje živý dashboard přes HTTP.

## Architektura

Všechno komunikuje přes jeden interní MQTT broker (`mqtt_broker`), který
funguje jako event bus mezi nezávisle nasaditelnými službami. Data protékají
třemi stupni, kdy každý publikuje na topic, který odebírá ten další:

```
[ingestery]  --input/#-->  [parsery]  --/data/{iot,web}/#-->  [storager]  -->  TimescaleDB
raw-input                  iot-parser                                          |
mqtt-inputer                web-parser                                         v
openmeteo-input                                                          api (HTTP) --> frontend + Caddy
```

- **Ingestery** (`cmd/raw-input`, `cmd/mqtt-inputer`, `cmd/openmeteo-input`,
  `cmd/owm-input`) dostávají data do systému zvenčí (surový TCP socket,
  přemostěný externí MQTT broker, nebo weather API) a publikují je,
  zabalená v podepsané obálce, na `input/...`.
- **Parsery** (`cmd/iot-parser`, `cmd/web-parser`) odebírají `input/#`,
  normalizují payload do jednotného tvaru a přeposílají ho dál na
  `/data/iot/...` nebo `/data/web/...`.
- **`cmd/storager`** odebírá `/data/iot/#` a `/data/web/#` a všechno ukládá
  do TimescaleDB.
- **`cmd/api`** servíruje uložená data a stav jednotlivých komponent jako
  JSON, a zároveň poslouchá na `status/#`, aby věděl, kdo z komponent žije.
- **`cmd/frontend`** vykresluje HTML dashboard (Chart.js), který si data
  tahá z API přímo v prohlížeči.
- **Caddy** sedí před `frontend` a `api` jako jeden reverse-proxy vstupní
  bod (port 3800).

Každá dlouhoběžící služba navíc publikuje na `status/<client-id>` - při
připojení (retained "alive") i periodickým heartbeatem, a jako MQTT Last
Will má nastavené "offline" - takže `api` umí sledovat, kdo žije, bez
dotazování (pollingu).

Původní ručně kreslené schéma, ze kterého tohle vzniklo, je v
`architektura/dashoarderHL.png` / `.drawio`.

## Služby (`cmd/`)

| Služba | Role |
|---|---|
| `raw-input` | TCP socket ingester pro starší senzorové feedy typu `metrika:hodnota` |
| `mqtt-inputer` | Přemosťuje externí MQTT broker/topic do interní sběrnice |
| `openmeteo-input` | Pravidelně se ptá Open-Meteo API, publikuje jako IoT data |
| `owm-input` | Totéž, ale pro OpenWeatherMap (aktuálně nezapojeno v `docker-compose.yaml`) |
| `iot-parser` | Normalizuje surová IoT data (formát `metrika:hodnota` nebo odvozené z topicu) |
| `web-parser` | Normalizuje surová scrapovaná/web data |
| `storager` | Odebírá normalizovaná data, zapisuje do TimescaleDB |
| `api` | HTTP API: poslední události, zdraví jednotlivých komponent |
| `frontend` | Vykresluje HTML/JS dashboard |

## Použité návrhové vzory a proč

- **Factory** — `internal/storage/factory.go` (`NewStorage`) vybere podle
  configu implementaci `Storage` (`timescaledb` nebo `mock`), takže zbytek
  kódu závisí jen na rozhraní `Storage`, ne na konkrétním DB driveru. Díky
  tomu je `mock` použitelný pro lokální vývoj bez skutečného Postgresu.
- **Strategy / interface segregation** — `internal/storage/interface.go`
  definuje `Storage` jako jediný kontrakt, který splňují `TimescaleStorage`
  i `MockStorage`. Volající (`storager`, `api`) jsou napsaní proti tomu
  rozhraní a nikdy nevědí, kterou implementaci dostali.
- **Generika jako mini-framework** — `internal/config.LoadConfig[T any]`
  načte YAML soubor do libovolného typu configu `T` a pak na něj přes
  struct tagy (`caarlos0/env`) přeloží proměnné prostředí. Každá služba v
  `cmd/*` si definuje vlastní config strukturu (např. `MqttIngesterConfig`,
  `OpenWeatherConfig`), která vkládá sdílený `BaseConfig` - společná pole
  (log level, MQTT, secret key) se tak nemusí opakovat v každé službě
  zvlášť, a přitom si každá služba drží vlastní silně typovaná specifická
  pole.
- **Adapter** — `internal/platform/mqtt/client.go` obaluje surového
  `paho.mqtt.golang` klienta do vlastního typu `Client`, který přidává
  chování specifické pro naši appku (status topic, heartbeat, LWT), o kterém
  podkladová knihovna nic neví - zbytek kódu tak nikdy nesahá na `paho`
  přímo.
- **Pipes and filters** — řetězec ingester → parser → storager (viz
  Architektura výše) je klasický pipeline: každý stupeň je samostatný OS
  proces s jednou zodpovědností, který odebírá jeden topic a produkuje
  jiný. Stupně lze restartovat, nasazovat znovu nebo nahradit nezávisle na
  sobě.
- **Publish/subscribe jako integrační styl** — služby na sebe nikdy nevolají
  přímo; jediné, na čem každá služba závisí, je MQTT broker. Díky tomu může
  `docker-compose.yaml` nastartovat služby prakticky v libovolném pořadí
  (kromě healthcheck bran u brokeru/DB), aniž by musely znát adresy jedna
  druhé.
- **Heartbeat / dead man's switch** — `Client.StartHeartbeat` spolu s MQTT
  Last Will (`SetWill(..., "dead", ...)`) zajišťuje, že `HealthManager` v
  `api` pozná pád komponenty, i když spadne nečekaně, bez nutnosti
  explicitního shutdown handleru.

## Spuštění

```sh
cp .env.example .env   # doplnit skutečné hodnoty
docker compose up -d --build
```

Tohle nastartuje interní Mosquitto broker, instanci TimescaleDB (schéma z
`init.sql`), instanci Valkey (připravená, zatím ji žádná služba nevyužívá),
všechny appkové služby výše a Caddy na `:3800`.

Porty (změna přes `.env.example`): frontend+api přes Caddy na `3800`, `api`
přímo na `8080`, `frontend` přímo na `8180`, Postgres na `5433` (schválně
mimo výchozích `5432`, protože ten bývá běžně obsazený), Mosquitto na
`1883`, Valkey na `6380`.

`mqtt-inputer` navíc přemosťuje feed z DS18B20 čidel
(`/msh/internal_temp/ds1`/`ds2`), který publikuje zařízení v LAN přímo do
tohohle stejného Mosquitto brokeru - pokud tenhle zdroj nemáš a chceš ho
vypnout, viz `SOURCE_MQTT_URL`/`SOURCE_MQTT_TOPIC` níže. Všechny služby,
které tenhle projekt potřebuje, včetně toho brokeru, jsou v tomhle
`docker-compose.yaml` - žádná závislost na stacku jiného projektu.

### Konfigurace (`.env`)

V `.env` žijí všechny secrets a hodnoty specifické pro dané prostředí; je v
`.gitignore` a nesmí se nikdy commitnout. `.env.example` je commitnutý
vzor bez secrets - zkopíruj ho do `.env` a doplň skutečné hodnoty. Cokoliv,
co není přebité skutečnou proměnnou prostředí, spadne zpátky na to, co je v
`.env.example`/`docker-compose.yaml`.

| Proměnná | Kdo ji čte | Význam |
|---|---|---|
| `MQTT_PORT` | `mqtt_broker` (port na hostu) | Port na hostu pro interní Mosquitto broker (kontejner uvnitř vždy poslouchá na `1883`) |
| `MQTT_BROKER_URL` | všechny appkové služby | Adresa interního brokeru, na kterou se appkové služby připojují, např. `tcp://mqtt_broker:1883` |
| `DB_USER` / `DB_PASSWORD` / `DB_NAME` | `timescale`, `api`, `storager` | Postgres role/databáze. Uplatní se jen při *prvním* startu TimescaleDB nad prázdným datovým adresářem - pozdější změna vyžaduje čerstvý `/srv/go-dashboarder/pgsql-data` |
| `VALKEY_PORT` / `VALKEY_PASSWORD` | `valkey` (port na hostu) | Port na hostu a heslo pro Valkey (připravená, zatím ji žádná služba nevyužívá) |
| `SOURCE_MQTT_URL` | `mqtt-inputer` | Adresa *externího* brokeru, ze kterého se přemosťuje (ne toho interního) |
| `SOURCE_MQTT_TOPIC` | `mqtt-inputer` | Topic (wildcards ok, např. `/msh/internal_temp/#`), který se na tom externím brokeru odebírá |
| `SECRET_KEY` | všechny appkové služby | Sdílený HMAC klíč pro podepisování/ověřování zpráv mezi ingestery a parsery - musí být všude stejný |
| `LOG_LEVEL` | většina služeb | Výchozí úroveň logování (`debug`/`info`/`warn`/...) |

### Build samostatných binárek (např. pro FreeBSD)

`build.sh` zkřížově zkompiluje staticky slinkovanou binárku pro každou
komponentu v `cmd/` a spolu s `configs/` a `web/` ji uloží do
`_release/<GOOS>-<GOARCH>/`:

```sh
./build.sh                          # výchozí je freebsd/amd64
GOOS=freebsd GOARCH=arm64 ./build.sh
GOOS=linux   GOARCH=arm64 ./build.sh
```

Je napsaný v POSIX `/bin/sh` (žádné bashismy), protože FreeBSD ve výchozím
stavu bash nemá. Výsledný adresář `_release/<cíl>/` zkopíruj na cílový
stroj a binárky spouštěj zevnitř něj (očekávají `configs/` a `web/` vedle
sebe, stejně jako když jsou zabuildované do Docker image).

## Známé nedodělky

- `cmd/owm-input` (OpenWeatherMap) je nahrazený `cmd/openmeteo-input`
  (Open-Meteo, nepotřebuje API klíč) a není zapojený v `docker-compose.yaml`.
- Valkey je v `docker-compose.yaml` připravená, ale zatím ji nevyužívá
  žádná služba.
- `postgresql-old.sql` je ponechaný jako historická reference na schéma, ze
  kterého tohle vzniklo; skutečně se aplikuje `init.sql`.
