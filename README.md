# ChirpStack FUOTA Server

## Build instructions

It is recommended to execute the following commands within the Docker Compose
development environment shell. To start this shell:

```
docker-compose run --rm chirpstack-fuota-server bash
```

```
# cleanup workspace
make clean

# run the tests
make test

# compile (this will also compile the ui and generate the static files)
make build

# compile snapshot builds for supported architectures (this will also compile the ui and generate the static files)
make snapshot
```
