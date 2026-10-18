FROM --platform=$BUILDPLATFORM node:24-alpine@sha256:44fce696f38ba8c987c4db89954051000bbd05ffd8d8659ba15ed4a8602e937f AS frontend
WORKDIR /src
COPY package.json package-lock.json ./
RUN npm ci --ignore-scripts --no-audit --no-fund && npm audit --audit-level=high
COPY web/styles ./web/styles
COPY web/templates ./web/templates
RUN ./node_modules/.bin/tailwindcss -i ./web/styles/input.css -o ./web/static/css/app.css --minify

FROM --platform=$BUILDPLATFORM golang:1.27.2-alpine@sha256:f92b6ef800e499660581efdabdf25d9d817a9d124eaf900924f0504e7e27e12d AS build
ARG TARGETOS
ARG TARGETARCH
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
COPY --from=frontend /src/web/static/css/app.css ./web/static/css/app.css
RUN CGO_ENABLED=0 GOOS=${TARGETOS:-linux} GOARCH=${TARGETARCH:-arm64} go build -trimpath -ldflags='-s -w' -o /out/leanprice ./cmd/leanprice \
    && mkdir -p /rootfs/data /rootfs/etc \
    && chmod 0750 /rootfs/data \
    && printf 'leanprice:x:65532:65532:leanPrice:/data:/sbin/nologin\n' > /rootfs/etc/passwd \
    && printf 'leanprice:x:65532:\n' > /rootfs/etc/group

FROM scratch AS runtime
WORKDIR /app
COPY --from=build --chown=65532:65532 /out/leanprice /app/leanprice
COPY --from=build --chown=65532:65532 /rootfs/data /data
COPY --from=build /rootfs/etc/passwd /etc/passwd
COPY --from=build /rootfs/etc/group /etc/group
COPY --from=build /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/ca-certificates.crt
ENV LEANPRICE_ADDR=0.0.0.0:8080 \
    LEANPRICE_DATA_DIR=/data \
    GOMEMLIMIT=256MiB \
    GOGC=100 \
    GOMAXPROCS=2
USER 65532:65532
EXPOSE 8080
VOLUME ["/data"]
HEALTHCHECK --interval=30s --timeout=3s --start-period=10s --retries=3 \
  CMD ["/app/leanprice", "healthcheck"]
ENTRYPOINT ["/app/leanprice"]
