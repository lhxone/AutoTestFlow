FROM harbor.inspur.local/atf/node:20-alpine AS builder
RUN sed -i 's/dl-cdn.alpinelinux.org/mirrors.aliyun.com/g' /etc/apk/repositories
RUN npm config set registry https://registry.npmmirror.com
WORKDIR /build
COPY package*.json ./
RUN npm ci --include=dev
COPY . .
RUN npm run build
FROM harbor.inspur.local/atf/nginx:1.27-alpine
COPY --from=builder /build/dist /usr/share/nginx/html
COPY nginx.conf /etc/nginx/conf.d/default.conf
EXPOSE 80
