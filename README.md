# Kubernetes Cloud Controller Manager for metal

metal-ccm is the Kubernetes cloud controller manager implementation for Metal.

## Deploy

Read how to deploy the metal CCM [here](deploy/releases/)!

## Building

To build the binary, run:

```bash
make build
```

It will deposit the binary for your local architecture as `dist/bin/metal-cloud-controller-manager-$(ARCH)`

By default `make build` builds the binary using a docker container. To install using your locally installed go toolchain, do:

```bash
make build LOCALBUILD=true
```

## Docker Image

To build a docker image, run:

```bash
make dockerimage
```

The image will be tagged with `:latest`.
