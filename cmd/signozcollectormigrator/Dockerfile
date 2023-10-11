# use a minimal alpine image
FROM alpine:3.17

# add ca-certificates in case you need them
RUN apk update && apk add ca-certificates && rm -rf /var/cache/apk/*

# define arguments and default values
ARG TARGETOS TARGETARCH
ARG USER_UID=10001

# create a non-root user for running the migrator
USER ${USER_UID}

# copy the binaries from the multi-stage build
COPY .build/${TARGETOS}-${TARGETARCH}/signoz-collector-migrator /signoz-collector-migrator

# run the binary as the entrypoint and pass the default dsn as a flag
ENTRYPOINT [ "/signoz-collector-migrator" ]
CMD ["--dsn", "tcp://host.docker.internal:9000"]
