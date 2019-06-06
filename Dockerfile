FROM alpine:3.9 as certs

RUN apk --update add ca-certificates

# Create Docker image of just the binary
FROM scratch as runner

ARG BINARY=metal-cloud-controller-manager
ARG ARCH=amd64

COPY --from=certs /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/ca-certificates.crt
COPY bin/${BINARY}-${ARCH} ${BINARY}

# because you cannot use ARG or ENV in CMD when in [] mode, and with "FROM scratch", we have no shell
CMD ["./metal-cloud-controller-manager"]
