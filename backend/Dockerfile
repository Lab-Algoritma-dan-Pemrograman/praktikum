# Go API + compiler C native, tetap dikelola Vercel tanpa VPS.
FROM golang:1.24-bookworm AS builder
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/server ./cmd/server

FROM debian:trixie-slim
RUN apt-get update && apt-get install -y --no-install-recommends \
    ca-certificates clang-19 lld-19 wasi-libc libclang-rt-19-dev-wasm32 \
    && rm -rf /var/lib/apt/lists/*
ENV PATH="/usr/lib/llvm-19/bin:${PATH}"
# Gagal saat build bila toolchain WASI belum lengkap, bukan saat ujian berjalan.
RUN printf '#include <stdio.h>\nint main(void) { puts("ok"); return 0; }\n' > /tmp/check.c \
    && clang-19 --target=wasm32-wasi --sysroot=/usr /tmp/check.c -o /tmp/check.wasm \
    && rm /tmp/check.c /tmp/check.wasm
RUN useradd --system --uid 10001 --create-home --shell /usr/sbin/nologin app
WORKDIR /app
COPY --from=builder --chown=app:app /out/server /app/server
ENV PORT=8080 \
    APP_ENV=production \
    CWASM_CLANG=/usr/bin/clang-19 \
    WASI_SYSROOT=/usr \
    CWASM_MAX_CONCURRENT=1
USER app
EXPOSE 8080
CMD ["/app/server"]
