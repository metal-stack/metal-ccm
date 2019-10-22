FROM golang:1.13-buster as builder
WORKDIR /work
COPY . .
RUN make all

FROM alpine:3.10
RUN apk --update add ca-certificates

COPY --from=builder /work/bin/metal-cloud-controller-manager .

CMD ["./metal-cloud-controller-manager"]
