# ---- build stage ----
FROM golang:1.26-alpine AS build
WORKDIR /src

# Cache dependencies first.
COPY go.mod go.sum ./
RUN go mod download

COPY . .
# Pure-Go build (modernc.org/sqlite needs no CGO) -> static binary.
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /server .

# ---- runtime stage ----
FROM alpine:3.20
# ca-certificates is required to verify HTTPS targets' certificates.
RUN apk add --no-cache ca-certificates \
 && adduser -D -u 10001 app \
 && mkdir -p /data && chown app /data

COPY --from=build /server /usr/local/bin/server

USER app
VOLUME /data
EXPOSE 8080
ENTRYPOINT ["/usr/local/bin/server"]
