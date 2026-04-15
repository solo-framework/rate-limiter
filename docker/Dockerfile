FROM golang:1.26.1-alpine3.23 AS builder
ARG VERSION=dev
ARG COMMIT=none
ARG DATE=none

RUN apk add --no-cache ca-certificates

WORKDIR /build

COPY . .

RUN go install github.com/golang/mock/mockgen@v1.6.0 \
	&& go mod download \
	&& go mod tidy && \
	go generate ./...

RUN rm -rf /out && mkdir -p /out \
		&& CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
			go build \
				-trimpath \
				-ldflags "-s -w -X main.version=$VERSION -X main.commit=$COMMIT -X main.date=$DATE" \
				-o /out/ratelimiter /build/cmd \
		&& cp /build/dist.config.toml /out/config.toml


FROM alpine:3.23

ENV RATE_LIMITER_ENV=production
ENV RATE_LIMITER_PORT=49105
ENV RATE_LIMITER_HTTP_PORT=8090
ENV RATE_LIMITER_STORE_PATH=/data

RUN apk --no-cache add ca-certificates tzdata

RUN addgroup -g 1000 rate && \
    adduser -D -u 1000 -G rate rate

RUN mkdir -p /app /data && \
    chown -R rate:rate /app /data

WORKDIR /app

COPY --from=builder --chown=rate:rate /out/ratelimiter /app/ratelimiter
COPY --from=builder --chown=rate:rate /out/config.toml /app/config.toml

RUN chmod +x /app/ratelimiter

USER rate
EXPOSE 49105 8090

HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
    CMD wget --quiet --tries=1 --spider http://localhost:$RATE_LIMITER_HTTP_PORT/health || exit 100

CMD ["/app/ratelimiter", "--config", "/app/config.toml"]
