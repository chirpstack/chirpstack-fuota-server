.PHONY: requirements api

PROTOC_ARGS := -I=/googleapis -I=../proto --go_out=. --go_opt=paths=source_relative --go-grpc_out=. --go-grpc_opt=paths=source_relative

all: requirements api

requirements:
	go mod download
	go install google.golang.org/protobuf/cmd/protoc-gen-go
	go install google.golang.org/grpc/cmd/protoc-gen-go-grpc

api:
	protoc ${PROTOC_ARGS} fuota.proto
