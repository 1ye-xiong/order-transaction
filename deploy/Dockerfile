FROM node:24-alpine AS frontend
WORKDIR /src/web/frontend
RUN corepack enable && corepack prepare pnpm@10.2.0 --activate
COPY web/frontend/package.json web/frontend/pnpm-lock.yaml ./
RUN pnpm install --frozen-lockfile --ignore-scripts
COPY web/frontend/ ./
RUN pnpm exec tsc --noEmit && pnpm build

# 构建六个服务二进制到 /app
FROM golang:1.25-alpine AS build
WORKDIR /src
ARG GOPROXY=https://proxy.golang.org,direct
ENV GOPROXY=${GOPROXY}
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
RUN apk add --no-cache tzdata ca-certificates && adduser -D -u 10001 appuser \
 && mkdir -p /app/data /app/logs && chown -R appuser:appuser /app/data /app/logs
COPY --from=build /out/ /app/
COPY configs/ /app/configs/
COPY web/index.html /app/web/index.html
COPY --from=frontend /src/web/dist/ /app/web/dist/
USER appuser
EXPOSE 8080
HEALTHCHECK NONE
CMD ["/app/gateway"]
