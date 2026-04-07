FROM scratch
ARG ALPINE_3_19_ROOTFS=artifacts/rootfs/alpine-minirootfs-3.19.0-x86_64.tar.gz
ADD ${ALPINE_3_19_ROOTFS} /
CMD ["/bin/sh"]
