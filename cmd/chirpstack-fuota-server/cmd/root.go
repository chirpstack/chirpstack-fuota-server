package cmd

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"reflect"
	"strings"

	"github.com/chirpstack/chirpstack-fuota-server/v4/internal/config"
	"github.com/mitchellh/mapstructure"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var (
	cfgFile string
	version string
)

var rootCmd = &cobra.Command{
	Use:   "chirpstack-fuota-server",
	Short: "ChirpStack FUOTA Server",
	RunE:  run,
}

func init() {
	cobra.OnInitialize(initConfig)

	rootCmd.PersistentFlags().StringVarP(&cfgFile, "config", "c", "", "path to configuration file (optional)")
	rootCmd.PersistentFlags().Int("log-level", 4, "debug=5, info=4, error=2, fatal=1, panic=0")

	viper.BindPFlag("general.log_level", rootCmd.PersistentFlags().Lookup("log-level"))

	// default values
	// viper.SetDefault("postgresql.dsn", "postgres://localhost/chirpstack_fuota?sslmode=disable")
	viper.SetDefault("postgresql.dsn", "postgres://chirpstack_fuota:dbpassword@localhost/chirpstack_fuota?sslmode=disable")
	viper.SetDefault("postgresql.automigrate", true)
	viper.SetDefault("postgresql.max_idle_connections", 2)

	viper.SetDefault("chirpstack.event_handler.marshaler", "protobuf")
	viper.SetDefault("chirpstack.event_handler.http.bind", "0.0.0.0:8090")
	viper.SetDefault("chirpstack.api.server", getChirpstackServer())
	// viper.SetDefault("chirpstack.api.token", "eyJ0eXAiOiJKV1QiLCJhbGciOiJIUzI1NiJ9.eyJhdWQiOiJjaGlycHN0YWNrIiwiaXNzIjoiY2hpcnBzdGFjayIsInN1YiI6IjMyMjA4NDczLWNkZjktNGUzYi05M2JjLTBjOTRkMTQ4ZDI0NiIsInR5cCI6ImtleSJ9.STkoqRjpHFz2kDtyd09BqNrh8mHk4RojismyXbygTv8")
	viper.SetDefault("chirpstack.api.token", getChirpstackToken())
	viper.SetDefault("chirpstack.api.tls_enabled", false)
	viper.SetDefault("fuota_server.api.bind", "0.0.0.0:8070")

	rootCmd.AddCommand(configCmd)
	rootCmd.AddCommand(versionCmd)
}

// Execute executes the root command.
func Execute(v string) {
	version = v

	if err := rootCmd.Execute(); err != nil {
		log.Fatal(err)
	}
}

func initConfig() {
	config.Version = version

	if cfgFile != "" {
		b, err := ioutil.ReadFile(cfgFile)
		if err != nil {
			log.WithError(err).WithField("config", cfgFile).Fatal("error loading config file")
		}
		viper.SetConfigType("toml")
		if err := viper.ReadConfig(bytes.NewBuffer(b)); err != nil {
			log.WithError(err).WithField("config", cfgFile).Fatal("error loading config file")
		}
	} else {
		viper.SetConfigName("chirpstack-fuota-server")
		viper.AddConfigPath(".")
		viper.AddConfigPath("$HOME/.config/chirpstack-fuota-server")
		viper.AddConfigPath("/etc/chirpstack-fuota-server")
		viper.AddConfigPath("/usr/local/bin")
		if err := viper.ReadInConfig(); err != nil {
			switch err.(type) {
			case viper.ConfigFileNotFoundError:
			default:
				log.WithError(err).Fatal("read configuration file error")
			}
		}
	}

	viperBindEnvs(config.C)

	viperHooks := mapstructure.ComposeDecodeHookFunc(
		viperDecodeJSONSlice,
		mapstructure.StringToTimeDurationHookFunc(),
		mapstructure.StringToSliceHookFunc(","),
	)

	if err := viper.Unmarshal(&config.C, viper.DecodeHook(viperHooks)); err != nil {
		log.WithError(err).Fatal("unmarshal config error")
	}
}

func viperBindEnvs(iface interface{}, parts ...string) {
	ifv := reflect.ValueOf(iface)
	ift := reflect.TypeOf(iface)
	for i := 0; i < ift.NumField(); i++ {
		v := ifv.Field(i)
		t := ift.Field(i)
		tv, ok := t.Tag.Lookup("mapstructure")
		if !ok {
			tv = strings.ToLower(t.Name)
		}
		if tv == "-" {
			continue
		}

		switch v.Kind() {
		case reflect.Struct:
			viperBindEnvs(v.Interface(), append(parts, tv)...)
		default:
			// Bash doesn't allow env variable names with a dot so
			// bind the double underscore version.
			keyDot := strings.Join(append(parts, tv), ".")
			keyUnderscore := strings.Join(append(parts, tv), "__")
			viper.BindEnv(keyDot, strings.ToUpper(keyUnderscore))
		}
	}
}

func viperDecodeJSONSlice(rf reflect.Kind, rt reflect.Kind, data interface{}) (interface{}, error) {
	// input must be a string and destination must be a slice
	if rf != reflect.String || rt != reflect.Slice {
		return data, nil
	}

	raw := data.(string)

	// this decoder expects a JSON list
	if !strings.HasPrefix(raw, "[") || !strings.HasSuffix(raw, "]") {
		return data, nil
	}

	var out []map[string]interface{}
	err := json.Unmarshal([]byte(raw), &out)

	return out, err
}

func getChirpstackToken() string {

	viper.SetConfigName("c2int_boot_config")
	viper.SetConfigType("toml")
	viper.AddConfigPath("/usr/local/bin")

	if err := viper.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); ok {
			log.Fatalf("c2int_boot_config.toml file not found: %v", err)
		} else {
			log.Fatalf("Error reading c2int_boot_config.toml file: %v", err)
		}
	}

	// Get the bearer token
	bearerToken := viper.GetString("chirpstack.bearer_token")
	if bearerToken == "" {
		log.Fatal("Bearer token not found in c2int_boot_config.toml file")
	}

	return bearerToken
}

func getChirpstackServer() string {

	viper.SetConfigName("c2int_boot_config")
	viper.SetConfigType("toml")
	viper.AddConfigPath("/usr/local/bin")

	if err := viper.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); ok {
			log.Fatalf("c2int_boot_config.toml file not found: %v", err)
		} else {
			log.Fatalf("Error reading c2int_boot_config.toml file: %v", err)
		}
	}

	host := viper.GetString("chirpstack.host")
	if host == "" {
		log.Fatal("host not found in c2int_boot_config.toml file")
	}

	port := viper.GetInt("chirpstack.port")
	if port == 0 {
		log.Fatal("port not found in c2int_boot_config.toml file")
	}

	serverUrl := fmt.Sprintf("%s:%d", host, port)

	return serverUrl
}
