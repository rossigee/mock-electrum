FROM golang:1.26.5-alpine AS builder
WORKDIR /build
COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 go build -ldflags="-s -w" -o electrum-mock ./cmd/main

FROM scratch
COPY --from=builder /build/electrum-mock /app
COPY --from=builder /build/fixtures /fixtures
COPY --from=builder /build/config.yaml /config.yaml
EXPOSE 50001 8081
ENTRYPOINT ["/app"]