# Multi-stage: build the static Go binary, ship it on a Node base (the
# chromecookies auth helper shells out to `node`/`npm`).
#
# ⚠️ AUTH LIMITATION (documented, not a bug): chromecookies decrypts the
# *host* Chrome cookie store via the host OS keychain. Inside a container
# there is no macOS Keychain / login keyring, so the auto-auth path will NOT
# work for a macOS host. Practical container use:
#   - run the READ commands with a pre-obtained, valid config.json mounted:
#       docker run --rm -v "$HOME/Library/Application Support/boursocli:/cfg" \
#         boursocli --config /cfg/config.json accounts
#   - or mount a Linux Chrome 'Cookies' DB and use --chrome-profile.
# No secret is baked into the image; credentials are always mounted at runtime.

FROM golang:1.25.10 AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
ARG VERSION=docker
ARG COMMIT=none
ARG DATE=unknown
RUN CGO_ENABLED=0 go build -trimpath \
    -ldflags "-s -w \
      -X github.com/thomasmarcelin754/boursocli/internal/version.Version=${VERSION} \
      -X github.com/thomasmarcelin754/boursocli/internal/version.Commit=${COMMIT} \
      -X github.com/thomasmarcelin754/boursocli/internal/version.Date=${DATE}" \
    -o /out/boursocli ./cmd/boursocli

FROM node:22-slim
RUN useradd -m app
COPY --from=build /out/boursocli /usr/local/bin/boursocli
USER app
WORKDIR /home/app
ENTRYPOINT ["boursocli"]
CMD ["--help"]
