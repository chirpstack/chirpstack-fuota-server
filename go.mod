module github.com/brocaar/chirpstack-fuota-server

go 1.15

require (
	github.com/brocaar/chirpstack-api/go/v3 v3.8.2-0.20201204140046-2f82b77c510f
	github.com/brocaar/lorawan v0.0.0-20201030140234-f23da2d4a303
	github.com/gofrs/uuid v3.3.0+incompatible
	github.com/golang-migrate/migrate/v4 v4.14.1
	github.com/golang/protobuf v1.4.3
	github.com/goreleaser/goreleaser v0.106.0
	github.com/goreleaser/nfpm v0.11.0
	github.com/grpc-ecosystem/go-grpc-middleware v1.2.2
	github.com/jmoiron/sqlx v1.2.1-0.20190826204134-d7d95172beb5
	github.com/lib/pq v1.9.0
	github.com/mitchellh/mapstructure v1.3.3
	github.com/pkg/errors v0.9.1
	github.com/rakyll/statik v0.1.7
	github.com/sirupsen/logrus v1.7.0
	github.com/spf13/cobra v1.1.1
	github.com/spf13/viper v1.7.1
	github.com/stretchr/testify v1.6.1
	google.golang.org/grpc v1.33.1
)
