# Majesta One API / worker — Go static binary + guest Node for customer automations (ADR-005 / ADR-034)
#
#   docker build -f deploy/Dockerfile --build-arg CMD=api -t one-api:go .
#   docker build -f deploy/Dockerfile --build-arg CMD=worker -t one-worker:go .
#
FROM golang:1.25-bookworm AS build
WORKDIR /src
ARG CMD=api
ARG VERSION=0.1.0
ARG NODE_VERSION=22.22.2
# BuildKit supplies TARGETARCH for the resolved image platform. Do not give it a
# default: that would silently install an amd64 Node binary into native arm64
# images when this Dockerfile is built outside a multi-platform buildx command.
ARG TARGETARCH
COPY go.mod go.sum ./
RUN go mod download
COPY cmd ./cmd
COPY internal ./internal
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath \
  -ldflags="-s -w -X github.com/MajestaNet/ide/internal/version.Version=${VERSION}" \
  -o /out/one ./cmd/${CMD}
# Pin Node guest executor (install-side only; not a platform runtime language).
# 22.22.2 has in-process module.registerHooks so the guest can deny worker threads.
RUN apt-get update && apt-get install -y --no-install-recommends curl ca-certificates \
  && rm -rf /var/lib/apt/lists/* \
  && arch="$TARGETARCH" \
  && if [ "$arch" = "amd64" ] || [ "$arch" = "x86_64" ]; then node_arch=x64; \
     elif [ "$arch" = "arm64" ] || [ "$arch" = "aarch64" ]; then node_arch=arm64; \
     else echo "unsupported arch: $arch" >&2; exit 1; fi \
  && curl -fsSL "https://nodejs.org/dist/v${NODE_VERSION}/node-v${NODE_VERSION}-linux-${node_arch}.tar.gz" -o /tmp/node.tgz \
  && tar -xzf /tmp/node.tgz -C /tmp \
  && install -m 755 "/tmp/node-v${NODE_VERSION}-linux-${node_arch}/bin/node" /out/node \
  && /out/node --version

# cc image provides glibc and libstdc++ for the Node binary; Go binary remains static.
FROM gcr.io/distroless/cc-debian12:nonroot
COPY --from=build /out/one /one
COPY --from=build /out/node /usr/local/bin/node
# Kernel SQL migrations
COPY migrations /migrations
ENV MIGRATIONS_PATH=/migrations
ENV NODE_BIN=/usr/local/bin/node
USER nonroot:nonroot
EXPOSE 8080
ENTRYPOINT ["/one"]
