FROM mirror.gcr.io/library/node:25-alpine AS base
FROM ghcr.io/basecamp/kamal:latest AS git

FROM base

# Copy git binary/runtime from an existing local Alpine image so jobs don't need apk/apt at runtime.
COPY --from=git /usr/bin/git /usr/bin/git
COPY --from=git /usr/libexec/git-core /usr/libexec/git-core
COPY --from=git /usr/share/git-core /usr/share/git-core
COPY --from=git /usr/lib/libpcre2-8.so.0 /usr/lib/libpcre2-8.so.0
COPY --from=git /usr/lib/libpcre2-8.so.0.12.0 /usr/lib/libpcre2-8.so.0.12.0
COPY --from=git /usr/lib/libcurl.so* /usr/lib/
COPY --from=git /usr/lib/libcares.so* /usr/lib/
COPY --from=git /usr/lib/libnghttp2.so* /usr/lib/
COPY --from=git /usr/lib/libidn2.so* /usr/lib/
COPY --from=git /usr/lib/libpsl.so* /usr/lib/
COPY --from=git /usr/lib/libssl.so* /usr/lib/
COPY --from=git /usr/lib/libcrypto.so* /usr/lib/
COPY --from=git /usr/lib/libzstd.so* /usr/lib/
COPY --from=git /usr/lib/libbrotlidec.so* /usr/lib/
COPY --from=git /usr/lib/libbrotlicommon.so* /usr/lib/
COPY --from=git /usr/lib/libunistring.so* /usr/lib/

RUN npm install -g opencode-ai@1.2.10 opencode-linux-arm64@1.2.10 opencode-linux-arm64-musl@1.2.10 --no-audit --no-fund
RUN opencode --version

WORKDIR /workspace
