FROM gcr.io/distroless/static-debian13:nonroot

COPY bin/metal-cloud-controller-manager .

CMD ["./metal-cloud-controller-manager"]
