FROM golang:1.24-bookworm AS builder
WORKDIR /work
COPY . .
RUN make

FROM gcr.io/distroless/static-debian12:nonroot

COPY --from=builder /work/bin/metal-cloud-controller-manager .

CMD ["./metal-cloud-controller-manager"]
