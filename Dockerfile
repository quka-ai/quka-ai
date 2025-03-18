FROM golang:1.23.0 AS builder

ARG TARGETOS
ARG TARGETARCH

WORKDIR /app

# Copy modules manifests
COPY go.mod go.sum ./

# Download modules with cache
RUN go mod download

# Copy source code
COPY cmd/ cmd/
COPY app/ app/
COPY pkg/ pkg/
COPY tpls/ tpls/

# Start build
RUN CGO_ENABLED=0 GOOS=${TARGETOS} GOARCH=${TARGETARCH} go build -a -ldflags '-extldflags "-static"' -o _build/brew-api ./cmd/

FROM alpine:3.18
LABEL MAINTAINER=hey@brew.re

WORKDIR /app
COPY --from=builder /app/cmd/service/etc/service-default.toml /app/etc/service-default.toml
COPY --from=builder /app/_build/brew-api /app/brew-api
COPY --from=builder /app/_build/tpls /app/tpls

CMD ["/app/brew-api", "service", "-c", "/app/etc/service-default.toml"]
