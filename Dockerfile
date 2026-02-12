FROM golang:1.26-trixie AS builder
WORKDIR /work
COPY . .
RUN make

FROM gcr.io/distroless/static-debian13:nonroot

COPY --from=builder /work/bin/metal-cloud-controller-manager .

CMD ["./metal-cloud-controller-manager"]
