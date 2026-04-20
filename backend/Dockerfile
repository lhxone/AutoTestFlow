FROM golang:1.26.2 AS builder

WORKDIR /build

RUN go env -w GOPROXY=https://goproxy.cn,direct && \
    go env -w GOSUMDB=sum.golang.google.cn

COPY go.mod go.sum ./
RUN GODEBUG=x509ignoreCN=0 \
    go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o server cmd/server/main.go

FROM alpine:3.20

RUN sed -i 's/dl-cdn.alpinelinux.org/mirrors.aliyun.com/g' /etc/apk/repositories && \
    apk add --no-cache ca-certificates tzdata wget
ENV TZ=Asia/Shanghai

WORKDIR /app
COPY --from=builder /build/server /app/server
COPY configs/config.yaml /app/configs/config.yaml

EXPOSE 8080

CMD ["./server", "-config", "configs/config.yaml"]
