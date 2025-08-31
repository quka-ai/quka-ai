FROM docker.1ms.run/library/golang:1.25-alpine AS builder

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
RUN CGO_ENABLED=0 GOOS=${TARGETOS} GOARCH=${TARGETARCH} go build -a -ldflags '-extldflags "-static"' -o _build/quka ./cmd/

FROM docker.1ms.run/library/alpine:3.20
LABEL MAINTAINER=hey@quka.ai

WORKDIR /app
COPY --from=builder /app/cmd/service/etc/service-default.toml /app/etc/service-default.toml
COPY --from=builder /app/_build/quka /app/quka
COPY --from=builder /app/tpls /app/tpls

CMD ["/app/quka", "service", "-c", "/app/etc/service-default.toml"]
