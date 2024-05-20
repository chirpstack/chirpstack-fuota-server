package as

import (
	"context"
	"fmt"
	"time"

	grpc_logrus "github.com/grpc-ecosystem/go-grpc-middleware/logging/logrus"
	log "github.com/sirupsen/logrus"
	"google.golang.org/grpc"

	"github.com/chirpstack/chirpstack-fuota-server/v4/internal/config"
	"github.com/chirpstack/chirpstack/api/go/v4/api"
)

var (
	clientConn *grpc.ClientConn

	applicationClient    api.ApplicationServiceClient
	multicastGroupClient api.MulticastGroupServiceClient
	deviceClient         api.DeviceServiceClient
)

type APIToken string

func (a APIToken) GetRequestMetadata(ctx context.Context, url ...string) (map[string]string, error) {
	return map[string]string{
		"authorization": fmt.Sprintf("Bearer %s", a),
	}, nil
}

func (a APIToken) RequireTransportSecurity() bool {
	return false
}

// Setup handles the AS client setup.
func Setup(conf *config.Config) error {
	log.Info("client/as: setup application-server client")

	logrusEntry := log.NewEntry(log.StandardLogger())
	logrusOpts := []grpc_logrus.Option{
		grpc_logrus.WithLevels(grpc_logrus.DefaultCodeToLevel),
	}
	// fmt.Println("\n\n token: " + conf.ChirpStack.API.Token + "\n\n")

	opts := []grpc.DialOption{
		grpc.WithBlock(),
		grpc.WithPerRPCCredentials(APIToken(conf.ChirpStack.API.Token)),
		grpc.WithUnaryInterceptor(
			grpc_logrus.UnaryClientInterceptor(logrusEntry, logrusOpts...),
		),
	}

	if !conf.ChirpStack.API.TLSEnabled {
		opts = append(opts, grpc.WithInsecure())
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	client, err := grpc.DialContext(ctx, conf.ChirpStack.API.Server, opts...)
	if err != nil {
		return fmt.Errorf("dial application-server api error: %w", err)
	}

	clientConn = client

	applicationClient = api.NewApplicationServiceClient(clientConn)
	multicastGroupClient = api.NewMulticastGroupServiceClient(clientConn)
	deviceClient = api.NewDeviceServiceClient(clientConn)

	return nil
}

func ApplicationClient() api.ApplicationServiceClient {
	return applicationClient
}

func MulticastGroupClient() api.MulticastGroupServiceClient {
	return multicastGroupClient
}

func DeviceClient() api.DeviceServiceClient {
	return deviceClient
}

func SetDeviceClient(c api.DeviceServiceClient) {
	deviceClient = c
}
