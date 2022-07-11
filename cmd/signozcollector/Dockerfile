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

RUN cd cmd/signozcollector && go build -o /out/signozcollector

# Final stage
FROM scratch

ARG USER_UID=10001

USER ${USER_UID}

COPY --from=build /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/ca-certificates.crt
COPY --from=build /out/signozcollector /
COPY --from=build /src/exporter/clickhousetracesexporter/migrations /migrations

EXPOSE 4317 55680 55679
ENTRYPOINT ["/signozcollector"]
CMD ["--config", "/etc/otel/config.yaml"]
