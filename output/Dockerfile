FROM golang:1.19-alpine as build

RUN --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=0 go install -ldflags "-s -w -extldflags '-static'" github.com/go-delve/delve/cmd/dlv@latest

WORKDIR /go/src/app
COPY . .

RUN --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build \
    -trimpath \
    -o out/riverfish-output \
    -v \
    -ldflags="-X main.CommitHash=$(cat .git/$(cat .git/HEAD | cut -d' ' -f2) | cut -c1-8)" \
    # delve
    -gcflags "all=-N -l" \
    github.com/K3das/riverfish/output

FROM alpine

# some of this was taken from https://github.com/Zenika/alpine-chrome/blob/2b7717a82b6e164dbf95f98add7c5d957d3942c4/Dockerfile
RUN apk upgrade --no-cache \
    && apk add --no-cache \
        chromium ttf-freefont font-noto-emoji \
    && apk add --no-cache \
      --repository=https://dl-cdn.alpinelinux.org/alpine/edge/testing \
      font-wqy-zenhei

RUN addgroup -S appgroup && adduser -S appuser -G appgroup

COPY --from=build --chown=appuser:appgroup /go/bin/dlv /dlv
COPY --from=build --chown=appuser:appgroup /go/src/app/out/ /opt/app/

WORKDIR /opt/app
USER appuser


ENV CHROME_BIN=/usr/bin/chromium-browser \
    CHROME_PATH=/usr/lib/chromium/

ENTRYPOINT [ "/opt/app/riverfish-output" ]