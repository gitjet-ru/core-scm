FROM scratch
ARG ALPINE_3_23_ROOTFS=artifacts/rootfs/alpine-minirootfs-3.23.0-x86_64.tar.gz
ADD ${ALPINE_3_23_ROOTFS} /
ARG GO_DIST_TARBALL=artifacts/go/go1.26.1.linux-amd64.tar.gz
ADD ${GO_DIST_TARBALL} /usr/local/
ENV GOROOT=/usr/local/go
ENV GOPATH=/go
ENV PATH=/usr/local/go/bin:/go/bin:/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin
RUN mkdir -p /go/src /go/bin /go/pkg
WORKDIR /go
CMD ["/bin/sh"]
