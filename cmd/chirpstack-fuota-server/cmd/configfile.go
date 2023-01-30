package cmd

import (
	"os"
	"text/template"

	"github.com/chirpstack/chirpstack-fuota-server/v4/internal/config"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
)

const configTemplate = `[general]
  # Log level
  #
  # debug=5, info=4, warning=3, error=2, fatal=1, panic=0
  log_level={{ .General.LogLevel }}

  # Log to syslog.
  #
  # When set to true, log messages are being written to syslog.
  log_to_syslog={{ .General.LogToSyslog }}

  # gRPC default resolver scheme.
  #
  # Set this to "dns" for enabling dns round-robin load balancing.
  grpc_default_resolver_scheme="{{ .General.GRPCDefaultResolverScheme }}"


# PostgreSQL settings.
# 
# Please note that PostgreSQL 9.5+ is required with the 'hstore' extension
# enabled.
[postgresql]

  # PostgreSQL dsn (e.g.: postgres://user:password@hostname/database?sslmode=disable).
  #
  # Besides using an URL (e.g. 'postgres://user:password@hostname/database?sslmode=disable')
  # it is also possible to use the following format:
  # 'user=chirpstack_ns dbname=chirpstack_ns sslmode=disable'.
  #
  # The following connection parameters are supported:
  #
  # * dbname - The name of the database to connect to
  # * user - The user to sign in as
  # * password - The user's password
  # * host - The host to connect to. Values that start with / are for unix domain sockets. (default is localhost)
  # * port - The port to bind to. (default is 5432)
  # * sslmode - Whether or not to use SSL (default is require, this is not the default for libpq)
  # * fallback_application_name - An application_name to fall back to if one isn't provided.
  # * connect_timeout - Maximum wait for connection, in seconds. Zero or not specified means wait indefinitely.
  # * sslcert - Cert file location. The file must contain PEM encoded data.
  # * sslkey - Key file location. The file must contain PEM encoded data.
  # * sslrootcert - The location of the root certificate file. The file must contain PEM encoded data.
  #
  # Valid values for sslmode are:
  #
  # * disable - No SSL
  # * require - Always SSL (skip verification)
  # * verify-ca - Always SSL (verify that the certificate presented by the server was signed by a trusted CA)
  # * verify-full - Always SSL (verify that the certification presented by the server was signed by a trusted CA and the server host name matches the one in the certificate)
  dsn="{{ .PostgreSQL.DSN }}"


  # Automatically apply database migrations.
  #
  # It is possible to apply the database-migrations by hand
  # (see https://github.com/brocaar/chirpstack-fuota-server/tree/master/migrations)
  # or let ChirpStack FUOTA Server migrate to the latest state automatically, by using
  # this setting. Make sure that you always make a backup when upgrading ChirpStack
  # FUOTA Server and / or applying migrations.
  automigrate={{ .PostgreSQL.Automigrate }}

  # Max open connections.
  #
  # This sets the max. number of open connections that are allowed in the
  # PostgreSQL connection pool (0 = unlimited).
  max_open_connections={{ .PostgreSQL.MaxOpenConnections }}

  # Max idle connections.
  #
  # This sets the max. number of idle connections in the PostgreSQL connection
  # pool (0 = no idle connections are retained).
  max_idle_connections={{ .PostgreSQL.MaxIdleConnections }}


# Application Server (integration) settings.
[application_server]

  # Event handler integration settings.
  [application_server.event_handler]

    # Payload marshaler.
    #
    # This defines how the HTTP payloads are encoded. Valid options are:
    # * protobuf:  Protobuf encoding
    # * json:      JSON encoding (easier for debugging, but less compact than 'protobuf')
    marshaler="{{ .ApplicationServer.EventHandler.Marshaler }}"

    # HTTP handler settings.
    [application_server.event_handler.http]

      # IP:Port to bind the event handler server to.
      bind="{{ .ApplicationServer.EventHandler.HTTP.Bind }}"

  # API integration settings.
  [application_server.api]

    # ChirpStack Application Server API server endpoint.
    server="{{ .ApplicationServer.API.Server }}"

    # API token.
    token=".ApplicationServer.API.Token"

    # Endpoint uses TLS.
    tls_enabled={{ .ApplicationServer.API.TLSEnabled }}


# FUOTA server settings.
[fuota_server]

  # FUOTA server API settings.
  [fuota_server.api]

    # IP:Port to bind the FUOTA server API to.
    bind="{{ .FUOTAServer.API.Bind }}"

    # Optional TLS configuration.
    ca_cert="{{ .FUOTAServer.API.CACert }}"
    tls_cert="{{ .FUOTAServer.API.TLSCert }}"
    tls_key="{{ .FUOTAServer.API.TLSKey }}"

`

var configCmd = &cobra.Command{
	Use:   "configfile",
	Short: "Print the ChirpStack FUOTA Server configuration file",
	RunE: func(cmd *cobra.Command, args []string) error {
		t := template.Must(template.New("config").Parse(configTemplate))
		err := t.Execute(os.Stdout, &config.C)
		if err != nil {
			return errors.Wrap(err, "execute config template error")
		}
		return nil
	},
}
