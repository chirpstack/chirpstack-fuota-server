FROM golang:1.19.5-alpine AS development

ENV PROJECT_PATH=/chirpstack-fuota-server
ENV PATH=$PATH:$PROJECT_PATH/build
ENV CGO_ENABLED=0
ENV GO_EXTRA_BUILD_ARGS="-a -installsuffix cgo"

RUN apk add --no-cache ca-certificates tzdata make git bash

RUN mkdir -p $PROJECT_PATH
COPY . $PROJECT_PATH
WORKDIR $PROJECT_PATH

RUN make dev-requirements
RUN make

FROM alpine:3.17.1 AS production

RUN apk --no-cache add ca-certificates tzdata
COPY --from=development /chirpstack-fuota-server/build/chirpstack-fuota-server /usr/bin/chirpstack-fuota-server
ENTRYPOINT ["/usr/bin/chirpstack-fuota-server"]
