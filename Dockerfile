FROM golang:1.23-bookworm as builder
WORKDIR /work
COPY . .
RUN make

FROM alpine:3.20
RUN apk --update add ca-certificates

COPY --from=builder /work/bin/metal-cloud-controller-manager .

CMD ["./metal-cloud-controller-manager"]
