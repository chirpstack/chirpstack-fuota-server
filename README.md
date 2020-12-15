# ChirpStack FUOTA Server

ChirpStack FUOTA Server is an open-source FUOTA server implementation for
LoRaWAN. It integrates with the [ChirpStack Application Server](https://www.chirpstack.io/application-server/)
using the HTTP integration (for receiving uplink payloads) and uses the
ChirpStack Application Server API for creating the multicast-groups and
enqueueing the downlink payloads.

## Building from source

The following commands explain how to compile the source-code using the
provided [Docker Compose](https://docs.docker.com/compose/) development
environment.

To start a `bash` shell within this environment:

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

## API interface

The ChirpStack FUOTA Server provides a [gRPC](https://grpc.io/) API interface
for scheduling the FUOTA deployments to one or multiple devices under a
ChirpStack Application Server application ID. This API is defined by the
gRPC [FUOTAServerService](https://github.com/brocaar/chirpstack-api/blob/master/protobuf/fuota/fuota.proto).

## Setup

After installing the ChirpStack FUOTA Server, there are a couple of steps to
take to setup. Example commands:

### Database setup

The ChirpStack FUOTA Server stores the deployment results into a [PostgreSQL](https://www.postgresql.org/)
database. You must enable the `hstore` extension for this database.

```bash
sudo -u postgres psql
```

```sql
-- set up the user and password
-- (note that it is important to use single quotes and a semicolon at the end!)
create role chirpstack_fuota with login password 'dbpassword';

-- create the database
create database chirpstack_fuota with owner chirpstack_fuota;

-- change to the ChirpStack FUOTA Server database
\c chirpstack_fuota

-- enable the hstore extension
create extension hstore;

-- exit psql
\q
```

### Create API key

The ChirpStack FUOTA Server needs a ChirpStack Application Server API in order
authenticate with the ChirpStack Application Server. You can generate this key
within the ChirpStack Application Server web-interface. This key must be
configured in the `chirpstack-fuota-server.toml` configuration file.

### HTTP integration

Within the ChirpStack Application Server, you must also setup a HTTP integration
for the application(s) you want to use with the ChirpStack FUOTA Server. Make
sure that this matches with the host / IP and port the ChirpStack FUOTA Server
event handler is binding to. As well make sure that the marshaler matches.

## Usage and configuration file

Please refer to the CLI interface for usage information and for the command to
generate a configuration file:

```
ChirpStack FUOTA Server

Usage:
  chirpstack-fuota-server [flags]
  chirpstack-fuota-server [command]

Available Commands:
  configfile  Print the ChirpStack FUOTA Server configuration file
  help        Help about any command
  version     Print the ChirpStack FUOTA Server version

Flags:
  -c, --config string   path to configuration file (optional)
  -h, --help            help for chirpstack-fuota-server
      --log-level int   debug=5, info=4, error=2, fatal=1, panic=0 (default 4)

Use "chirpstack-fuota-server [command] --help" for more information about a command.
```

## Resources

For a better understanding of FUOTA, please refer to the following documents
that can be found at the [LoRa Alliance Resource Hub](https://lora-alliance.org/resource-hub):

* Remote Multicast Setup
* Fragmented Data Block Transport

## License

ChirpStack FUOTA Server is distributed under the MIT license. See also
[LICENSE](https://github.com/brocaar/chirpstack-fuota-server/blob/master/LICENSE).
