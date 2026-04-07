FROM scratch
ARG ALPINE_3_23_ROOTFS=artifacts/rootfs/alpine-minirootfs-3.23.0-x86_64.tar.gz
ADD ${ALPINE_3_23_ROOTFS} /
CMD ["/bin/sh"]
