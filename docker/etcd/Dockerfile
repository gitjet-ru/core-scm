# etcd v3.4.x из официального релиза (linux-${TARGETARCH}).
# Нужен v3.4: Patroni в Spilo использует API v2 (в 3.5+ снят). В compose задайте --enable-v2=true (в 3.4.x по умолчанию false).
# Сборка под архитектуру хоста избегает эмуляции amd64 на arm64 (mmap bbolt → "cannot allocate memory").
FROM alpine:3.19

ARG ETCD_VERSION=3.4.33
ARG TARGETARCH

RUN apk add --no-cache ca-certificates curl \
	&& curl -fsSL "https://github.com/etcd-io/etcd/releases/download/v${ETCD_VERSION}/etcd-v${ETCD_VERSION}-linux-${TARGETARCH}.tar.gz" \
	| tar -xz \
	&& mv "etcd-v${ETCD_VERSION}-linux-${TARGETARCH}/etcd" /usr/local/bin/ \
	&& mv "etcd-v${ETCD_VERSION}-linux-${TARGETARCH}/etcdctl" /usr/local/bin/ \
	&& rm -rf "etcd-v${ETCD_VERSION}-linux-${TARGETARCH}"

COPY entrypoint.sh /entrypoint.sh
RUN chmod +x /entrypoint.sh

ENTRYPOINT ["/entrypoint.sh"]
CMD ["etcd"]
