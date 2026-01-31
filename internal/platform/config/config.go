package config

import (
	"encoding/json"
	"os"
	"strconv"
	"time"
)

const (
	ProgramVersion     = "v0.04" /* aktualni verze programu se promitne i do verze api */
	DefaultHost        = "127.0.0.1"
	DefaultMqttPort    = 1883
	DefaultDbPort      = 5432
	DefaultApiPort     = 3400
	DefaultWebPort     = 5000
	DefaultQueuSize    = 1024
	DefaultQueuPolicy  = "block"
	DefaultListenIp    = "0.0.0.0"
	DefaultMaxConn     = 25
	DefaultTls         = false
	DefaultVerifyTls   = false
	DefaultAccessToken = "zhhPwsxUbnN+TzL3mlJgCvmkqrPBr/bTWvwwjfYne1C/rXUXaCY5"
)

/* Konfigurace pro platformu */
type Config struct {
	LogLevel    string                     `json:"log_level,omitempty"`                 /* Volitelne - v podstate string -> slog.LogLevel, aka : Debug/Info/Warn/Error, default Info pokud neni specifikovano */
	MqttInputs  []MqttConf                 `json:"mqtt_inputs_config"`                  /* Povinne - pole konfiguraci jednotlivych mqtt vstupu */
	OtherInputs map[string]json.RawMessage `json:"alternative_inputs_config,omitempty"` /* Volitelne - zde pripadnde vlozime dalsi konfigurace pro jine vstupy dat */
	MqttIntern  MqttConf                   `json:"mqtt_internal_server"`                /* Povinne - konfigurace vnitrniho "bridge" - pouzivame vlastni mqtt server pro predaveni dat mezi komponentami */
	Queue       QueueConf                  `json:"queue_config"`                        /* Povinne - vnitrni queue pokud potrebujeme rychle predani a nestaci nam vnitrni mqtt server (napriklad mezi channely) */
	ApiSrv      ApiConf                    `json:"api_config"`                          /* Povinne - konfigurace naseho api serveru */
	WebSrv      WebConf                    `json:"web_config"`                          /* Povinne - nas web server ktery poskytuje stranky generovane z templates, ktere se napoji na api server */
	Databases   []DbConf                   `json:"databases_config,omitempty"`          /* Volitelne - konfigurace nami vyuzitych databasi - konfig je "jednotny" a obsahuje typ */
}

/* vlastni konfigurace interni queue */
type QueueConf struct {
	Size           int    `json:"queue_size"`   /* velikost vnitrni queue, by default 1024 */
	OverflowPolicy string `json:"queue_policy"` /* primarne block/drop/spill_to_disk/ded-leatter, defaulkt block */
}

/* konfigurace pro api server */
type ApiConf struct {
	ListenAddr string `json:"listen_address"`  /* ip adresa kde ma api poslouchat */
	ListenPort int    `json:"listen_port"`     /* port kde ma api posloucaht, defaultne 3400/tcp */
	MaxConn    int    `json:"max_connections"` /* maximalni pocet pripojeni na api, defaultne 25 */
	Version    string `json:"api_version"`     /* api version , aktualne se bere z constanty programversion v config.go */
}

/* konfigurace mqtt - general setting, pouzitelne nasobne */
type MqttConf struct {
	Name              string        `json:"conn_name"`                  /* povinne - pojmenovat instanci kterou konnektime, jinak se v tom nevyzname */
	ConnectURL        string        `json:"conn_url"`                   /* povinny parameter, connection url je : tcp://[fqdn]:port, nebo ssl://[fqdn]:port */
	User              string        `json:"conn_user,omitempty"`        /* muze byt prazdny podud server podporuje anonymni connection */
	Pass              string        `json:"conn_pass,omitempty"`        /* muze byt prazdny podud server podporuje anonymni connection */
	ClientID          string        `json:"conn_clientid"`              /* je povinny parametr ke kezde konexi a musi byt unikatni, jinak se nam to bude nekde stekat ! */
	Topics            []string      `json:"conn_topic"`                 /* povinne - topics na subscribe */
	UseTLS            bool          `json:"conn_tls,omitempty"`         /* pokud je zde true, url musi byt ssl:// a resime i tls parametry, jinak je neresime */
	VerifyTLS         bool          `json:"conn_tls_verify,omitempty"`  /* pokud je zde false, vypneme kontrolu certifikatu na ssl vrstve (devel prostredi, nase prostredi se self-sign crt, apod). NEPOUZIVAT V PRODUKCI ! */
	UseMTLS           bool          `json:"conn_mtls,omitempty"`        /* mtls na spojeni ? pokud mame UseTLS a zde je false, neresime vlastni crt a key, pokud je zde true, MUSIME mit vlastni crt a key */
	CaCRTFile         string        `json:"ca_crt_file,omitempty"`      /* odkud nacteme CA soubor pro spojeni na mqtt */
	OwnCRTFile        string        `json:"own_crt_file,omitempty"`     /* odkud nacist vlastni crt kdyz mTLS */
	OwnCRTKeyFile     string        `json:"own_crt_key_file,omitempty"` /* odkud nacist vlastni klic k mTLS */
	ConnectionTimeout time.Duration `json:"conn_timeout,omitempty"`     /* timeout pro spojeni na mqtt, default 5s pokud neni specifikovano jinak */
	SubscribeTimeout  time.Duration `json:"subs_timeout,omitempty"`     /* timeout pro subscribe na mqtt, default 5s pokud neni specifikovano jinak */
}

/* konfigurace web serveru */
type WebConf struct {
	ServerAddr  string `json:"websrv_address"` /* povinne - na ktere adrese posloucha web server(frontend), backend je api server */
	ServerPort  int    `json:"websrv_ports"`   /* povinne - na kterem portu posloucha web server - default 5000 */
	AccessToken string `json:"websrv_token"`   /* povinne - access token pro pristup na konfiguracni parametry, prozatim jeden, do budoucna zmenime az budeme resit auth vrstvy */
}

/* konfigurace pro databsi - vyuzitelne pro ruzne db */
type DbConf struct {
	Dsn     string `json:"db_dsn,omitempty"`        /* connection string like: postgres://user:password@host:port/database */
	UseTls  bool   `json:"db_use_tls,omitempty"`    /* by default pouzivame tls spojeni, zde to lze vypnout. Nema smysl napriklad pro sqlite */
	MaxConn int    `json:"db_conn_limit,omitempty"` /* by default pocitame s max 25 spojenimmi na db, zde lze zmenit, nicmene treba u sqlite tohle nema smysl - v podstate konfigurace size conneciton poolu */
}

/* funkce pro konfiguraci */

// NewConfig - vrati instanci konfigurace s defaultnimi hodnotami
func NewConfig() *Config {
	// vytvorime instanci
	cfg := new(Config)
	// nahrajeme vychozi hodnoty
	cfg.ApplyDefaults()
	// vracime
	return cfg
}

// ApplyDefaults - nahrajeme vychozi hodnoty do nove struktury
func (c *Config) ApplyDefaults() {
	// nejdriv loglevel, default je info, mozno jeste debug/warn/error
	c.LogLevel = "info"
	// Vezmeme to postupne, zacneme internimi vecmi jako je queue
	c.Queue.Size = DefaultQueuSize
	c.Queue.OverflowPolicy = DefaultQueuPolicy
	// Web server konfigurace (frontend)
	c.WebSrv.ServerAddr = DefaultListenIp
	c.WebSrv.ServerPort = DefaultWebPort
	c.WebSrv.AccessToken = DefaultAccessToken // POZOR TOTO SE MUSI ZMENIT PRI RUNU, overime nez publikujeme conf
	// Database conf
	c.Databases = append(c.Databases, DbConf{
		Dsn:     "postgres://user:pass@127.0.0.1:5432/database",
		UseTls:  false,
		MaxConn: DefaultMaxConn,
	})
	// Api konfigurace
	c.ApiSrv.ListenAddr = DefaultListenIp
	c.ApiSrv.ListenPort = DefaultApiPort
	c.ApiSrv.MaxConn = DefaultMaxConn
	c.ApiSrv.Version = ProgramVersion

	// Interni mqtt server - vychozi hodnoty
	c.MqttIntern.Name = "Internal"
	c.MqttIntern.ConnectURL = "tcp://127.0.0.1:1883"
	c.MqttIntern.ClientID = "dashboarder_base_0-04"
	c.MqttIntern.User = ""
	c.MqttIntern.Pass = ""
	c.MqttIntern.Topics = []string{"/dashboarder/internal"}
	c.MqttIntern.ConnectionTimeout = 5 * time.Second
	c.MqttIntern.SubscribeTimeout = 5 * time.Second
	c.MqttIntern.UseTLS = false
	c.MqttIntern.UseMTLS = false
	c.MqttIntern.VerifyTLS = false

	// Input mqtt nastavime prozatim prazdne
	c.MqttInputs = []MqttConf{}

} // konec ApplyDefaults

/* nyni implementujeme nacitaci funkce - nejdrive ty pomocne */
// GetEnvStr - precte string z env a vrati obsah nebo vrati default
func GetEnvStr(envName string, defval string) string {
	if val := os.Getenv(envName); val != "" {
		return val
	}
	return defval
}

// GetEnvInt - precte int a vrati nebo vrati default, nebo 0 kdyz nastane chyba !
func GetEnvInt(envName string, defval int) int {
	if val := os.Getenv(envName); val != "" {
		rval, err := strconv.Atoi(val)
		if err != nil {
			return 0
		}
		return rval
	}
	return defval
}
