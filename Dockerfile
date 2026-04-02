# Build static binary (modernc.org/sqlite is pure Go; CGO not required).
FROM golang:1.22-alpine AS build
WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/konkon ./cmd/konkon

FROM alpine:3.20
RUN apk add --no-cache ca-certificates tzdata su-exec wget \
	&& addgroup -g 65532 -S konkon \
	&& adduser -u 65532 -S -G konkon konkon

ENV KONKON_DATA_DIR=/data \
	KONKON_LISTEN=:8080

RUN mkdir -p /data/uploads

COPY docker-entrypoint.sh /docker-entrypoint.sh
RUN chmod +x /docker-entrypoint.sh

COPY --from=build /out/konkon /usr/local/bin/konkon

WORKDIR /data
EXPOSE 8080
ENTRYPOINT ["/docker-entrypoint.sh"]
