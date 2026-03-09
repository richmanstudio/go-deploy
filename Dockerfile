# Multi-stage Dockerfile for building and running go-deploy.
FROM golang:1.26-alpine AS builder

WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o /out/go-deploy ./cmd/go-deploy

FROM alpine:3.22 AS certs
RUN apk --no-cache add ca-certificates

FROM scratch

WORKDIR /app
COPY --from=builder /out/go-deploy /go-deploy
COPY --from=certs /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/ca-certificates.crt

ENTRYPOINT ["/go-deploy"]
