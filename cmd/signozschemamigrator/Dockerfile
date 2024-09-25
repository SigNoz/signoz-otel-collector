# use a minimal alpine image

ARG ALPINE_SHA="pass-a-valid-docker-sha-otherwise-this-will-fail"

FROM alpine:3.17@sha256:${ALPINE_SHA}

# define arguments and default values
ARG USER_UID=10001
ARG OS="linux"
ARG ARCH

# create a non-root user for running the migrator
USER ${USER_UID}

# copy the binaries from the multi-stage build
COPY ./target/${OS}-${ARCH}/signoz-schema-migrator /signoz-schema-migrator
COPY migrationmanager ./migrationmanager

# run the binary as the entrypoint and pass the default dsn as a flag
ENTRYPOINT [ "/signoz-schema-migrator" ]
CMD ["--dsn", "tcp://localhost:9000"]
