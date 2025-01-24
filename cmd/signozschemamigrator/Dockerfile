# use a minimal alpine image
FROM alpine:3.17

# define arguments and default values
ARG TARGETOS TARGETARCH
ARG USER_UID=10001

# create a non-root user for running the migrator
USER ${USER_UID}

# copy the binaries from the multi-stage build
COPY .build/${TARGETOS}-${TARGETARCH}/signoz-schema-migrator /usr/local/bin/signoz-schema-migrator
COPY cmd/signozschemamigrator/run.sh /run.sh

# run both sync and async schema migrations in sequence using script
ENTRYPOINT [ "/run.sh" ]
CMD ["all"]
