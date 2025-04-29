FROM golang:1.24.2-alpine3.21 AS builder

ARG BUILD_DATE
ARG BUILD_VERSION

ENV CGO_ENABLED=0

WORKDIR /app

COPY go.mod go.sum ./

RUN go mod download

COPY . .

RUN DT="${BUILD_DATE}" \
    VER="${BUILD_VERSION:-latest}" \
    go build -o quartz -ldflags "-s -w -X main.version=$VER -X main.buildDate=$DT" ./cmd/quartz/main.go

FROM alpine:3.21

WORKDIR /app

COPY --from=builder /app/quartz .

ENTRYPOINT ["./quartz"]
CMD ["--help"]
