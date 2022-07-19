# Builder stage
FROM --platform=$BUILDPLATFORM golang:1.17-alpine as build
RUN apk --update add ca-certificates

WORKDIR /src
COPY . .

ARG TARGETOS TARGETARCH

ENV OS111MODULE=on
ENV CGO_ENABLED=0
ENV GOOS=$TARGETOS
ENV GOARCH=$TARGETARCH

RUN cd cmd/signozcollector && go build -o /out/signoz-collector

# Final stage
FROM scratch

ARG USER_UID=10001

USER ${USER_UID}

COPY --from=build /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/ca-certificates.crt
COPY --from=build /out/signoz-collector /
COPY --from=build /src/exporter/clickhousetracesexporter/migrations /migrations

EXPOSE 4317 4318
ENTRYPOINT ["/signoz-collector"]
CMD ["--config", "/etc/otel/config.yaml"]
