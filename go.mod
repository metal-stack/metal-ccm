module github.com/metal-stack/metal-ccm

go 1.16

require (
	github.com/NYTimes/gziphandler v1.1.1 // indirect
	github.com/google/uuid v1.3.0
	github.com/gorilla/websocket v1.4.2 // indirect
	github.com/metal-stack/metal-go v0.15.7
	github.com/metal-stack/metal-lib v0.8.1
	github.com/metal-stack/v v1.0.3
	github.com/onsi/ginkgo v1.16.4 // indirect
	github.com/spf13/pflag v1.0.5
	k8s.io/api v0.20.1
	k8s.io/apimachinery v0.20.1
	k8s.io/client-go v0.20.1
	k8s.io/cloud-provider v0.19.10
	k8s.io/component-base v0.20.1
	k8s.io/kubernetes v1.19.11
	sigs.k8s.io/yaml v1.2.0
)

replace (
	github.com/go-logr/logr => github.com/go-logr/logr v0.4.0
	// specify a lower transitive dependency to grpc otherwise
	// endpoint.go:114:78: undefined: resolver.BuildOption
	google.golang.org/grpc => google.golang.org/grpc v1.26.0
	gopkg.in/square/go-jose.v2 => gopkg.in/square/go-jose.v2 v2.2.2
	k8s.io/api => k8s.io/api v0.19.10
	k8s.io/apiextensions-apiserver => k8s.io/apiextensions-apiserver v0.19.10
	k8s.io/apimachinery => k8s.io/apimachinery v0.19.10
	k8s.io/apiserver => k8s.io/apiserver v0.19.10
	k8s.io/cli-runtime => k8s.io/cli-runtime v0.19.10
	k8s.io/client-go => k8s.io/client-go v0.19.10
	k8s.io/cloud-provider => k8s.io/cloud-provider v0.19.10
	k8s.io/cluster-bootstrap => k8s.io/cluster-bootstrap v0.19.10
	k8s.io/code-generator => k8s.io/code-generator v0.19.10
	k8s.io/component-base => k8s.io/component-base v0.19.10
	k8s.io/cri-api => k8s.io/cri-api v0.19.10
	k8s.io/csi-translation-lib => k8s.io/csi-translation-lib v0.19.10
	k8s.io/kube-aggregator => k8s.io/kube-aggregator v0.19.10
	k8s.io/kube-controller-manager => k8s.io/kube-controller-manager v0.19.10
	k8s.io/kube-proxy => k8s.io/kube-proxy v0.19.10
	k8s.io/kube-scheduler => k8s.io/kube-scheduler v0.19.10
	k8s.io/kubectl => k8s.io/kubectl v0.19.10
	k8s.io/kubelet => k8s.io/kubelet v0.19.10
	k8s.io/legacy-cloud-providers => k8s.io/legacy-cloud-providers v0.19.10
	k8s.io/metrics => k8s.io/metrics v0.19.10
	k8s.io/sample-apiserver => k8s.io/sample-apiserver v0.19.10
)
