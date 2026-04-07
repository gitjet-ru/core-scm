# etcd v3.4.x из официального релиза (linux-${TARGETARCH}).
# Нужен v3.4: Patroni в Spilo использует API v2 (в 3.5+ снят). В compose задайте --enable-v2=true (в 3.4.x по умолчанию false).
# Сборка под архитектуру хоста избегает эмуляции amd64 на arm64 (mmap bbolt → "cannot allocate memory").
ARG ALPINE_BASE_REF=REGISTRY_BASE/alpine-base@sha256:REPLACE_ALPINE_3_19_DIGEST
FROM ${ALPINE_BASE_REF}

ARG ETCD_VERSION=3.4.33
ARG TARGETARCH

RUN apk add --no-cache ca-certificates curl \
	&& ETCD_ARCHIVE="etcd-v${ETCD_VERSION}-linux-${TARGETARCH}.tar.gz" \
	&& curl -fsSL "https://storage.googleapis.com/etcd/v${ETCD_VERSION}/${ETCD_ARCHIVE}" -o "${ETCD_ARCHIVE}" \
	&& curl -fsSL "https://storage.googleapis.com/etcd/v${ETCD_VERSION}/SHA256SUMS" -o SHA256SUMS \
	&& grep " ${ETCD_ARCHIVE}$" SHA256SUMS > SHA256SUMS.filtered \
	&& test -s SHA256SUMS.filtered \
	&& sha256sum -c SHA256SUMS.filtered \
	&& tar -xzf "${ETCD_ARCHIVE}" \
	&& mv "etcd-v${ETCD_VERSION}-linux-${TARGETARCH}/etcd" /usr/local/bin/ \
	&& mv "etcd-v${ETCD_VERSION}-linux-${TARGETARCH}/etcdctl" /usr/local/bin/ \
	&& rm -rf "etcd-v${ETCD_VERSION}-linux-${TARGETARCH}" "${ETCD_ARCHIVE}" SHA256SUMS SHA256SUMS.filtered

COPY entrypoint.sh /entrypoint.sh
RUN chmod +x /entrypoint.sh

ENTRYPOINT ["/entrypoint.sh"]
CMD ["etcd"]
