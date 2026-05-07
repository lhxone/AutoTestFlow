ARG NODE_IMAGE=harbor.inspur.local/atf/node:20-alpine
ARG NGINX_IMAGE=harbor.inspur.local/atf/nginx:1.27-alpine

FROM ${NODE_IMAGE} AS builder
ARG NPM_CONFIG_REGISTRY=https://registry.npmmirror.com
RUN sed -i 's/dl-cdn.alpinelinux.org/mirrors.aliyun.com/g' /etc/apk/repositories
RUN npm config set registry "$NPM_CONFIG_REGISTRY" \
  && npm config set fetch-retries 5 \
  && npm config set fetch-retry-mintimeout 20000 \
  && npm config set fetch-retry-maxtimeout 120000
WORKDIR /build
COPY package*.json ./
RUN npm ci --include=dev --no-audit --fund=false
COPY . .
RUN npm run build
FROM ${NGINX_IMAGE}
COPY --from=builder /build/dist /usr/share/nginx/html
COPY nginx.conf /etc/nginx/conf.d/default.conf
EXPOSE 80
