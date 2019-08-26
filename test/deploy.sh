#!/usr/bin/env bash

CURR_DIR=$(pwd)
function finish {
  cd ${CURR_DIR}
}
trap finish EXIT
TEST_DIR=$( dirname $(readlink -f ${BASH_SOURCE[0]} ))

if [[ ! -z ${GOPATH} ]]; then
  cd ${GOPATH}
  go get sigs.k8s.io/kind@v0.5.0
fi

METAL_API_URL=${METAL_API_URL}
if [[ -z ${METAL_API_URL} ]]; then
  METAL_API_URL=${METALCTL_URL-http://metal.test.fi-ts.io}
fi

METAL_API_HMAC=${METAL_API_HMAC}
if [[ -z ${METAL_API_HMAC} ]]; then
  METAL_API_HMAC=${METALCTL_HMAC-metal-test-admin}
fi

cd ${TEST_DIR}
cd ..

make clean gofmt bin/metal-cloud-controller-manager
docker build -f Dockerfile.local --no-cache -t metalpod/metal-ccm:v0.0.1 .

kind delete cluster 2>/dev/null
docker rm -f kind-control-plane 2>/dev/null

kind create cluster
export KUBECONFIG=$(kind get kubeconfig-path --name=kind)
kind load docker-image metalpod/metal-ccm:v0.0.1

echo "apiVersion: v1
kind: Secret
metadata:
  name: metal-cloud-config
  namespace: kube-system
stringData:
  apiUrl: \"$METAL_API_URL\"
  apiKey: \"\"
  apiHMAC: \"$METAL_API_HMAC\"" > test/metal-cloud-config.yaml

kubectl apply -f test/metal-cloud-config.yaml
kubectl apply -f test/rbac.yaml
kubectl apply -f test/metallb.yaml
kubectl apply -f deploy/releases/v0.0.1/deployment.yaml

echo "Don't forget to delete cluster after testing by running:"
echo "kind delete cluster"
