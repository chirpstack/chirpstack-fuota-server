package as

import (
	"context"
	"fmt"
	"time"

	grpc_logrus "github.com/grpc-ecosystem/go-grpc-middleware/logging/logrus"
	log "github.com/sirupsen/logrus"
	"google.golang.org/grpc"

	"github.com/brocaar/chirpstack-api/go/v3/as/external/api"
	"github.com/brocaar/chirpstack-fuota-server/internal/config"
)

var (
	clientConn *grpc.ClientConn

	applicationClient    api.ApplicationServiceClient
	multicastGroupClient api.MulticastGroupServiceClient
	deviceQueueClient    api.DeviceQueueServiceClient
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

	opts := []grpc.DialOption{
		grpc.WithBlock(),
		grpc.WithPerRPCCredentials(APIToken(conf.ApplicationServer.API.Token)),
		grpc.WithUnaryInterceptor(
			grpc_logrus.UnaryClientInterceptor(logrusEntry, logrusOpts...),
		),
	}

	if !conf.ApplicationServer.API.TLSEnabled {
		opts = append(opts, grpc.WithInsecure())
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	client, err := grpc.DialContext(ctx, conf.ApplicationServer.API.Server, opts...)
	if err != nil {
		return fmt.Errorf("dial application-server api error: %w", err)
	}

	clientConn = client

	applicationClient = api.NewApplicationServiceClient(clientConn)
	multicastGroupClient = api.NewMulticastGroupServiceClient(clientConn)
	deviceQueueClient = api.NewDeviceQueueServiceClient(clientConn)

	return nil
}

func ApplicationClient() api.ApplicationServiceClient {
	return applicationClient
}

func MulticastGroupClient() api.MulticastGroupServiceClient {
	return multicastGroupClient
}

func DeviceQueueClient() api.DeviceQueueServiceClient {
	return deviceQueueClient
}
