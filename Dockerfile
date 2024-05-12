ARG ARCH=amd64

FROM --platform=linux/${ARCH} alpine:latest as baseimage

RUN apk add nfs-utils

#Get the mount.nfs4 dependency
RUN ldd /sbin/mount.nfs4 | tr -s '[:space:]' '\n' | grep '^/' | xargs -I % sh -c 'mkdir -p /nfs-deps/$(dirname %) && cp -L % /nfs-deps/%'
RUN ldd /sbin/mount.nfs | tr -s '[:space:]' '\n' | grep '^/' | xargs -I % sh -c 'mkdir -p /nfs-deps/$(dirname %) && cp -r -u -L % /nfs-deps/%'

FROM --platform=linux/${ARCH} golang:1.22 as builder

RUN CGO_ENABLED=0 go install github.com/go-delve/delve/cmd/dlv@latest

FROM --platform=linux/${ARCH} gcr.io/distroless/static@sha256:9be3fcc6abeaf985b5ecce59451acbcbb15e7be39472320c538d0d55a0834edc

LABEL maintainers="The NetApp Trident Team" \
      app="trident.netapp.io" \
      description="Trident Storage Orchestrator"

COPY --from=baseimage /bin/mount /bin/umount /bin/
COPY --from=baseimage /sbin/mount.nfs /sbin/mount.nfs4 /sbin/
COPY --from=baseimage /etc/netconfig /etc/
COPY --from=baseimage /nfs-deps/ /
COPY --from=builder /go/bin/dlv /

ARG BIN=trident_orchestrator
ARG CLI_BIN=tridentctl
ARG CHWRAP_BIN=chwrap.tar

COPY ${BIN} /trident_orchestrator
COPY ${CLI_BIN} /bin/tridentctl

ADD ${CHWRAP_BIN} /

EXPOSE 40000

ENTRYPOINT ["/bin/tridentctl"]
CMD ["version"]
