FROM --platform=$BUILDPLATFORM golang:1.26.2-alpine3.23 AS builder

ARG TARGETOS
ARG TARGETARCH

ARG VERSION=dev
ARG COMMIT=none
ARG DATE=none

RUN apk add --no-cache ca-certificates

WORKDIR /build

# COPY go.mod go.sum
COPY ../ .

RUN go install github.com/golang/mock/mockgen@v1.6.0 \
	&& go install go.uber.org/mock/mockgen@latest \
	&& go install golang.org/x/vuln/cmd/govulncheck@latest \
	&& go mod download \
	&& go mod tidy \
	&& go generate ./... \
	&& govulncheck ./...

RUN rm -rf /out && mkdir -p /out \
		&& CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH \
			go build \
				-trimpath \
				-ldflags "-s -w -X main.version=$VERSION -X main.commit=$COMMIT -X main.date=$DATE" \
				-o /out/ratelimiter /build/cmd \
		&& cp /build/dist.config.toml /out/config.toml

FROM scratch AS release
COPY --from=builder /out /

