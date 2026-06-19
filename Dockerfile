FROM golang:1.26-alpine AS go-builder

WORKDIR /src
RUN apk update && apk add --no-cache git ca-certificates tzdata && update-ca-certificates

COPY server/go.mod server/go.sum ./
RUN go mod download

COPY server/ ./

RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/protocol-api ./api/protocol
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/admin-api ./api/admin
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/migrate ./db
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/asset-gc ./cmd/asset-gc

FROM alpine:3 AS api

RUN apk update && apk add --no-cache wget ca-certificates tzdata && update-ca-certificates
ENV TZ=Asia/Shanghai

WORKDIR /app

COPY --from=go-builder /out/protocol-api /out/admin-api /out/migrate /out/asset-gc ./
COPY server/api/protocol/etc/protocol-api.yaml ./etc/protocol-api.yaml
COPY server/api/admin/etc/admin-api.yaml ./etc/admin-api.yaml

EXPOSE 8080 8081

CMD ["./protocol-api", "-f", "./etc/protocol-api.yaml"]
