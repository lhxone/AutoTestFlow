ARG RUNTIME_IMAGE=harbor.inspur.local/atf/auto-test-flow-backend-runtime:node20-bash-chromium

FROM harbor.inspur.local/atf/golang:1.26.2 AS builder

WORKDIR /build

RUN go env -w GOPROXY=https://goproxy.cn,direct && \
    go env -w GOSUMDB=sum.golang.google.cn

COPY go.mod go.sum ./
RUN GODEBUG=x509ignoreCN=0 \
    go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o server cmd/server/main.go

FROM ${RUNTIME_IMAGE}

WORKDIR /app
COPY --from=builder /build/server /app/server
COPY configs/config.yaml /app/configs/config.yaml

EXPOSE 8080

CMD ["./server", "-config", "configs/config.yaml"]
