ARG RUNTIME_IMAGE=harbor.inspur.local/atf/auto-test-flow-backend-runtime:node20-bash-playwright-deps

FROM harbor.inspur.local/atf/golang:1.26.2 AS builder

WORKDIR /build

ARG GOPROXY=https://goproxy.cn|https://proxy.golang.org|https://mirrors.aliyun.com/goproxy|direct
ARG GOSUMDB=sum.golang.google.cn
ENV GOPROXY=${GOPROXY} \
    GOSUMDB=${GOSUMDB}

COPY go.mod go.sum ./
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    for i in 1 2 3; do \
      GODEBUG=x509ignoreCN=0 go mod download && exit 0; \
      sleep $((i * 5)); \
    done; \
    GODEBUG=x509ignoreCN=0 go mod download

COPY . .
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o server cmd/server/main.go

FROM ${RUNTIME_IMAGE}

WORKDIR /app
COPY --from=builder /build/server /app/server
COPY configs/config.yaml /app/configs/config.yaml

EXPOSE 8080

CMD ["./server", "-config", "configs/config.yaml"]
