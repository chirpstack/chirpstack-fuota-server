package config

// Version defines the ChirpStack FUOTA Server version.
var Version string

// Config defines the config structure.
type Config struct {
	General struct {
		LogLevel                  int    `mapstructure:"log_level"`
		LogToSyslog               bool   `mapstructure:"log_to_syslog"`
		GRPCDefaultResolverScheme string `mapstructure:"grpc_default_resolver_scheme"`
	} `mapstructure:"general"`

	PostgreSQL struct {
		DSN                string `mapstructure:"dsn"`
		Automigrate        bool   `mapstructure:"automigrate"`
		MaxOpenConnections int    `mapstructure:"max_open_connections"`
		MaxIdleConnections int    `mapstructure:"max_idle_connections"`
	} `mapstructure:"postgresql"`

	ApplicationServer struct {
		EventHandler struct {
			Marshaler string `mapstructure:"marshaler"`
			HTTP      struct {
				Bind string `mapstructure:"bind"`
			} `mapstructure:"http"`
		} `mapstructure:"event_handler"`

		API struct {
			Server     string `mapstructure:"server"`
			Token      string `mapstructure:"token"`
			TLSEnabled bool   `mapstructure:"tls_enabled"`
		} `mapstructure:"api"`
	} `mapstructure:"application_server"`

	FUOTAServer struct {
		API struct {
			Bind    string `mapstructure:"bind"`
			CACert  string `mapstructure:"ca_cert"`
			TLSCert string `mapstructure:"tls_cert"`
			TLSKey  string `mapstructure:"tls_key"`
		} `mapstructure:"api"`
	} `mapstructure:"fuota_server"`
}

// C holds the global configuration.
var C Config
