FROM golang:1.16-buster as builder
WORKDIR /work
COPY . .
RUN make all

FROM alpine:3.14
RUN apk --update add ca-certificates

COPY --from=builder /work/bin/metal-cloud-controller-manager .

CMD ["./metal-cloud-controller-manager"]
