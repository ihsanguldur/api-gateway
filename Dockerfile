FROM golang:1.25 AS build
WORKDIR /src
COPY go.mod ./
COPY cmd ./cmd
COPY internal ./internal
RUN CGO_ENABLED=0 go build -o /out/gateway ./cmd/gateway
RUN CGO_ENABLED=0 go build -o /out/mockbackend ./cmd/mockbackend

FROM gcr.io/distroless/static-debian12 AS gateway
COPY --from=build /out/gateway /gateway
COPY config.json /config.json
ENTRYPOINT ["/gateway", "--config", "/config.json"]

FROM gcr.io/distroless/static-debian12 AS mockbackend
COPY --from=build /out/mockbackend /mockbackend
ENTRYPOINT ["/mockbackend"]