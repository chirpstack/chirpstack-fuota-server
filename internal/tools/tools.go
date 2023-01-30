//go:build tools
// +build tools

package tools

import (
	_ "github.com/golang-migrate/migrate/v4/cmd/migrate"
	_ "github.com/golang/mock/mockgen"
	_ "github.com/goreleaser/goreleaser"
	_ "github.com/goreleaser/nfpm"
	_ "github.com/rakyll/statik"
	_ "google.golang.org/grpc/cmd/protoc-gen-go-grpc"
	_ "google.golang.org/protobuf/cmd/protoc-gen-go"
)
