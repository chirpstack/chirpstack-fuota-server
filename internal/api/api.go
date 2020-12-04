package api

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"io/ioutil"
	"net"

	"github.com/pkg/errors"
	log "github.com/sirupsen/logrus"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"

	"github.com/brocaar/chirpstack-api/go/v3/fuota"
	"github.com/brocaar/chirpstack-fuota-server/internal/config"
)

func Setup(conf *config.Config) error {
	apiConf := conf.FUOTAServer.API

	log.WithFields(log.Fields{
		"bind":     apiConf.Bind,
		"ca_cert":  apiConf.CACert,
		"tls_cert": apiConf.TLSCert,
		"tls_key":  apiConf.TLSKey,
	}).Info("api: starting fuota-server api server")

	opts := getServerOptions()

	if apiConf.CACert != "" || apiConf.TLSCert != "" || apiConf.TLSKey != "" {
		creds, err := getTransportCredentials(apiConf.CACert, apiConf.TLSCert, apiConf.TLSKey, true)
		if err != nil {
			return fmt.Errorf("get transport credentials error: %w", err)
		}

		opts = append(opts, grpc.Creds(creds))
	}

	gs := grpc.NewServer(opts...)
	fuotaAPI := NewFUOTAServerAPI()
	fuota.RegisterFUOTAServerServiceServer(gs, fuotaAPI)

	ln, err := net.Listen("tcp", apiConf.Bind)
	if err != nil {
		return fmt.Errorf("start api listener error: %w", err)
	}

	go gs.Serve(ln)

	return nil
}

func getServerOptions() []grpc.ServerOption {
	return []grpc.ServerOption{}
}

func getTransportCredentials(caCert, tlsCert, tlsKey string, verifyClientCert bool) (credentials.TransportCredentials, error) {
	cert, err := tls.LoadX509KeyPair(tlsCert, tlsKey)
	if err != nil {
		return nil, errors.Wrap(err, "load tls key-pair error")
	}

	var caCertPool *x509.CertPool
	if caCert != "" {
		rawCaCert, err := ioutil.ReadFile(caCert)
		if err != nil {
			return nil, errors.Wrap(err, "load ca certificate error")
		}

		caCertPool = x509.NewCertPool()
		if !caCertPool.AppendCertsFromPEM(rawCaCert) {
			return nil, fmt.Errorf("append ca certificate error: %s", caCert)
		}
	}

	if verifyClientCert {
		return credentials.NewTLS(&tls.Config{
			Certificates: []tls.Certificate{cert},
			ClientCAs:    caCertPool,
			ClientAuth:   tls.RequireAndVerifyClientCert,
		}), nil
	}

	return credentials.NewTLS(&tls.Config{
		Certificates: []tls.Certificate{cert},
		RootCAs:      caCertPool,
	}), nil
}
