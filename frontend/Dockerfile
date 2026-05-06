FROM harbor.inspur.local/atf/node:20-alpine AS builder

RUN sed -i 's/dl-cdn.alpinelinux.org/mirrors.aliyun.com/g' /etc/apk/repositories
ARG NPM_CONFIG_REGISTRY=https://registry.npmmirror.com
RUN npm config set registry ${NPM_CONFIG_REGISTRY} && \
    npm config set fetch-retries 5 && \
    npm config set fetch-retry-mintimeout 20000 && \
    npm config set fetch-retry-maxtimeout 120000
WORKDIR /build

COPY package*.json ./
RUN --mount=type=cache,target=/root/.npm,sharing=locked \
    set -eu; \
    for i in 1 2 3; do \
      rm -rf node_modules; \
      if npm ci --include=dev --no-audit --fund=false \
        && test -x node_modules/.bin/vue-tsc \
        && test -x node_modules/.bin/vite; then \
        exit 0; \
      fi; \
      npm cache verify || true; \
      sleep $((i * 10)); \
    done; \
    rm -rf node_modules; \
    npm ci --include=dev --no-audit --fund=false; \
    test -x node_modules/.bin/vue-tsc; \
    test -x node_modules/.bin/vite

COPY . .
RUN npm run build

FROM harbor.inspur.local/atf/nginx:1.27-alpine

COPY --from=builder /build/dist /usr/share/nginx/html
COPY nginx.conf /etc/nginx/conf.d/default.conf

EXPOSE 80
