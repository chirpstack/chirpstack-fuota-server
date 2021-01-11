.PHONY: build clean generate statics test snapshot dist dev-requirements

build:
	@echo "Compiling source"
	@mkdir -p build
	go build $(GO_EXTRA_BUILD_ARGS) -ldflags "-s -w -X main.version=$(VERSION)" -o build/chirpstack-fuota-server cmd/chirpstack-fuota-server/main.go

clean:
	@echo "Cleaning up workspace"
	@rm -rf build
	@rm -rf dist

generate: statics

statics:
	@echo "Generating static files"
	statik -src migrations/ -dest internal/ -p migrations -f

test:
	@echo "Running tests"
	go test -p 1 -v ./...

snapshot:
	@goreleaser --snapshot

dist:
	goreleaser
	mkdir -p dist/upload/tar
	mkdir -p dist/upload/deb
	mkdir -p dist/upload/rpm
	mv dist/*.tar.gz dist/upload/tar
	mv dist/*.deb dist/upload/deb
	mv dist/*.rpm dist/upload/rpm

dev-requirements:
	go install github.com/rakyll/statik
	go install github.com/golang-migrate/migrate/v4/cmd/migrate
	go install github.com/goreleaser/goreleaser
	go install github.com/goreleaser/nfpm
	go install github.com/golang/mock/mockgen
