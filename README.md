# Dashboarder

## Aplikace pro zobrazeni: 
    - IoT dat z nasich senzoru
    - IoT dat z externich zdroju
    - Dalsich dat dle pozadavku

## Aplikacni architektura:
    - Primarne se pouziva interni mqtt process (spawn mosquitto skrze docker-compose.yaml. servise mqtt) pro predavani dat mezi komponentami.
    - status/ - primarni strom pro key_val nazev_komponenty:alive (hearbeat), napr. status
    - data/ - primarni strom pro datove prenosy mezi komponentami
    - ext/ - primarni strom pro data z externich zdroju (nase ingestery nahraji "surova data" do prislusnych channelu
    - log/[komponenta] - komponenty primarne loguji do os.Stdout ale soucasne poslou logy na tyto channely. Je to pro pripad budouci integrace logovani "nekam dal"
    - vyuzivame interne factory pro tvorbu jednotlivych mqtt klientu (kazda komponenta minimalne posila komponenta:alive do status/)
    - vyuzivame singleton slog pro logovani komponent (kazda komponenta posila logy take do log/[komponenta]
    - kvuli moznym blokacim krizenim bezi kazda komponenta v goroutinach, aka minimalne 2 (heartbeat a vlastni komponentni sluzba)
    - konfigurace se nacita jednotne, minimalen interni mqtt je jednotne pro vsechny komponenty (az na clientID)

### Dataflow analyza
    - nacitame data ze zdroju (nektere jsou externi mqtt, nektere interni mqtt, nekde muze byt napriklad web page nebo snmp query)
    - data "obalime" univerzalni strukturou DataMsg { id uuid.UUID, received time.Time, payload interface{} }
    - data publikujeme dle konfigurace do prislusneho interniho channelu : data/[komponenta]/
    - v pripade napriklad vice channelu na jednom mqtt serveru ktere odebirame, udelame vice instanci, abychom si navzajem neblokovaly klienta
    - data z chanellu data/... odebiraji "parsery" ktere se snazi je rozlozit do univerzalni struktury pouzitelne v timescaledb
    - data po validnim parsovani jdou do prislusnych "hypertables" v timescaledb, v pripade chyby parsovani jdou do tabulky ErrParse => zde si strukturu dodelame pozdeji
    - data ulozena v prislusnych tabulich budou zpristupnena pres REST api (auth header - s tim pocitat)
    - jedna z mikroservis bude "frontend", alias web server ktery poskytne nami definovane stranky na zaklade templates, s minimalnim moznym javascriptem
    - stylovani stranek generovanych z templates bude pres css, web server poskytne /static/style.css
    - pristup ke strankam/api bude hlidat caddy reverse proxy -> basic auth pro pristup
    - publikovano bude skrze ssh tunneling pri reverzenim spojeni - primarne mikroservisy pobezi na nasi rpi doma, zpristupnime pouze web iface pres rproxy kterou mame na verejne dostupnem VM

### Data-Live analyza
    - jelikoz se jedna vetsinou o IoT data , neni potreba dlouhy cyklus ulozeni dat, reknem ze data lifetime prozatim bude 33 dni
    - po 33 dnech se data "presunou do zalohy" - mozna vyuzit valkeydb s lifetime pro "ziva data" a timescaledb pro trvale ulozeni
    - data o stavech sluzeb budou ciste "live" , poslednich 33 dni nam bohate staci (takze asi valkeydb s lifetime)



