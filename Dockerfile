FROM golang:1.23-bookworm AS builder
WORKDIR /work
COPY . .
RUN make

FROM gcr.io/distroless/static-debian12

COPY --from=builder /work/bin/metal-cloud-controller-manager .

CMD ["./metal-cloud-controller-manager"]
