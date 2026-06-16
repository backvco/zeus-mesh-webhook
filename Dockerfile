FROM --platform=$BUILDPLATFORM golang:1.23-alpine AS builder
ARG BUILDPLATFORM
ARG TARGETPLATFORM
ARG TARGETOS
ARG TARGETARCH
ARG VERSION=dev

WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH \
    go build -ldflags "-s -w -X main.version=${VERSION}" \
    -o /zeus-mesh-webhook ./cmd/webhook

FROM scratch
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY --from=builder /zeus-mesh-webhook /zeus-mesh-webhook
ENTRYPOINT ["/zeus-mesh-webhook"]
