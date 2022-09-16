FROM golang:1.19-buster as builder
WORKDIR /work
COPY . .
RUN make

FROM alpine:3.16
RUN apk --update add ca-certificates

COPY --from=builder /work/bin/metal-cloud-controller-manager .

CMD ["./metal-cloud-controller-manager"]
