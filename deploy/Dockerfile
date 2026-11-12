# 构建六个服务二进制到 /app
FROM golang:1.24-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -o /out/gateway ./cmd/gateway \
 && CGO_ENABLED=0 go build -o /out/user ./cmd/user \
 && CGO_ENABLED=0 go build -o /out/product ./cmd/product \
 && CGO_ENABLED=0 go build -o /out/order ./cmd/order \
 && CGO_ENABLED=0 go build -o /out/payment ./cmd/payment \
 && CGO_ENABLED=0 go build -o /out/warehouse ./cmd/warehouse

FROM alpine:3.20
WORKDIR /app
RUN apk add --no-cache tzdata ca-certificates && adduser -D -u 10001 appuser
COPY --from=build /out/ /app/
COPY configs/ /app/configs/
COPY web/ /app/web/
USER appuser
EXPOSE 8080
HEALTHCHECK --interval=10s --timeout=3s CMD wget -qO- http://127.0.0.1:8080/healthz || exit 1
CMD ["/app/gateway"]
