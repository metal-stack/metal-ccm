#!/usr/bin/env bash

set -e

CURR_DIR=$(pwd)
function finish {
  set +e
  kind delete cluster &>/dev/null
  cd "${CURR_DIR}"
}
trap finish EXIT
TEST_DIR=$( dirname "$(readlink -f "${BASH_SOURCE[0]}" )")

METAL_API_URL=${METAL_API_URL}
if [[ -z ${METAL_API_URL} ]]; then
  METAL_API_URL=${METALCTL_API_URL-http://metal.test.fi-ts.io}
fi

METAL_API_HMAC=${METAL_API_HMAC}
if [[ -z ${METAL_API_HMAC} ]]; then
  METAL_API_HMAC=${METALCTL_HMAC-metal-test-admin}
fi

cd "${TEST_DIR}"
go build -tags netgo -o bin/echo echo.go
docker build -f Dockerfile.echo -t metalpod/test:echo .
cd ..

make clean gofmt bin/metal-cloud-controller-manager
docker build -f Dockerfile.local --no-cache -t metalpod/metal-ccm:v0.0.1 .

echo "apiVersion: v1
kind: Secret
metadata:
  name: metal-cloud-config
  namespace: kube-system
stringData:
  apiUrl: \"$METAL_API_URL\"
  apiKey: \"\"
  apiHMAC: \"$METAL_API_HMAC\"" > test/metal-cloud-config.yaml

echo "kind create cluster --config test/kind-config.yaml"
kind create cluster --config test/kind-config.yaml
KUBECONFIG=$(kind get kubeconfig-path --name=kind)
export KUBECONFIG

echo "kind load docker-image metalpod/metal-ccm:v0.0.1"
kind load docker-image metalpod/metal-ccm:v0.0.1
echo "kind load docker-image metalpod/test:echo"
kind load docker-image metalpod/test:echo

echo "kubectl apply -f test/metal-cloud-config.yaml"
kubectl apply -f test/metal-cloud-config.yaml
echo "kubectl apply -f test/rbac.yaml"
kubectl apply -f test/rbac.yaml
echo "kubectl apply -f test/metallb.yaml"
kubectl apply -f test/metallb.yaml
echo "kubectl apply -f deploy/releases/v0.0.1/deployment.yaml"
kubectl apply -f deploy/releases/v0.0.1/deployment.yaml

echo "Wait for 35 seconds..."
sleep 35

echo "kubectl apply -f test/echo.yaml"
kubectl apply -f test/echo.yaml

echo "Wait for 10 seconds..."
sleep 10

echo "kubectl logs -n kube-system $(kubectl get pod -n kube-system | grep metal-cloud | cut -d' ' -f1)"
kubectl logs -n kube-system "$(kubectl get pod -n kube-system | grep metal-cloud | cut -d' ' -f1)"

echo "kubectl describe svc -n kube-system echo"
kubectl describe svc -n kube-system echo

echo "kubectl describe cm config -n metallb-system"
kubectl describe cm config -n metallb-system

echo "Test echo service via load balancer..."
LB_INGRESS_IP=$(kubectl describe svc -n kube-system echo | grep Ingress | cut -d: -f2 | sed -e 's/^[ \t]*//')
for i in {1..5}; do
  echo "docker exec -t kind-control-plane curl ${LB_INGRESS_IP}:8080/echo"
  docker exec -t kind-control-plane curl "${LB_INGRESS_IP}":8080/echo
done

echo

kubectl get pod -n kube-system | grep echo- | cut -d' ' -f1 | while IFS= read -r i; do
  echo "kubectl logs -n kube-system $i"
  kubectl logs -n kube-system "$i"
done
