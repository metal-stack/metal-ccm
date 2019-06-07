FROM golang:1.12-stretch as builder
WORKDIR /work
COPY go.mod .
RUN go mod download

COPY . .
RUN make all

FROM alpine:3.9
RUN apk --update add ca-certificates
ARG BINARY=metal-cloud-controller-manager

COPY --from=builder /work/bin/${BINARY} ${BINARY}

# because you cannot use ARG or ENV in CMD when in [] mode, and with "FROM scratch", we have no shell
CMD ["./metal-cloud-controller-manager"]
