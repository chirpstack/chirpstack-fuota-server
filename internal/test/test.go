package test

import (
	"os"

	"github.com/chirpstack/chirpstack-fuota-server/v4/internal/config"
	log "github.com/sirupsen/logrus"
)

func init() {
	log.SetLevel(log.ErrorLevel)
}

func GetConfig() config.Config {
	var c config.Config

	c.PostgreSQL.DSN = "postgres://localhost/chirpstack_fuota?sslmode=disable"
	if v := os.Getenv("TEST_POSTGRES_DSN"); v != "" {
		c.PostgreSQL.DSN = v
	}

	return c
}
