# Majesta One API / worker — Go static binary + guest Deno for customer automations (ADR-005 / ADR-014)
#
#   docker build -f deploy/Dockerfile --build-arg CMD=api -t one-api:go .
#   docker build -f deploy/Dockerfile --build-arg CMD=worker -t one-worker:go .
#
FROM golang:1.25-bookworm AS build
WORKDIR /src
ARG CMD=api
ARG VERSION=0.1.0
ARG DENO_VERSION=2.9.3
# BuildKit supplies TARGETARCH for the resolved image platform. Do not give it a
# default: that would silently install an amd64 Deno binary into native arm64
# images when this Dockerfile is built outside a multi-platform buildx command.
ARG TARGETARCH
COPY go.mod go.sum ./
RUN go mod download
COPY cmd ./cmd
COPY internal ./internal
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath \
  -ldflags="-s -w -X github.com/MajestaNet/ide/internal/version.Version=${VERSION}" \
  -o /out/one ./cmd/${CMD}
# Pin Deno guest executor (install-side only; not a platform runtime language).
RUN apt-get update && apt-get install -y --no-install-recommends curl unzip ca-certificates \
  && rm -rf /var/lib/apt/lists/* \
  && arch="$TARGETARCH" \
  && if [ "$arch" = "amd64" ] || [ "$arch" = "x86_64" ]; then deno_arch=x86_64; \
     elif [ "$arch" = "arm64" ] || [ "$arch" = "aarch64" ]; then deno_arch=aarch64; \
     else echo "unsupported arch: $arch" >&2; exit 1; fi \
  && curl -fsSL "https://github.com/denoland/deno/releases/download/v${DENO_VERSION}/deno-${deno_arch}-unknown-linux-gnu.zip" -o /tmp/deno.zip \
  && unzip -o /tmp/deno.zip -d /out \
  && chmod +x /out/deno \
  && /out/deno --version

# cc image provides glibc for the Deno binary; Go binary remains static.
FROM gcr.io/distroless/cc-debian12:nonroot
COPY --from=build /out/one /one
COPY --from=build /out/deno /usr/local/bin/deno
# Kernel SQL migrations
COPY migrations /migrations
ENV MIGRATIONS_PATH=/migrations
ENV DENO_PATH=/usr/local/bin/deno
USER nonroot:nonroot
EXPOSE 8080
ENTRYPOINT ["/one"]
