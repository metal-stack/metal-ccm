module github.com/metal-stack/metal-ccm

go 1.13

require (
	github.com/NYTimes/gziphandler v1.0.1 // indirect
	github.com/coreos/pkg v0.0.0-20180928190104-399ea9e2e55f // indirect
	github.com/ghodss/yaml v1.0.0
	github.com/go-openapi/analysis v0.19.6 // indirect
	github.com/go-openapi/jsonreference v0.19.3 // indirect
	github.com/go-openapi/loads v0.19.4 // indirect
	github.com/go-openapi/runtime v0.19.7 // indirect
	github.com/go-openapi/spec v0.19.4 // indirect
	github.com/go-openapi/validate v0.19.4 // indirect
	github.com/golang/groupcache v0.0.0-20180513044358-24b0969c4cb7 // indirect
	github.com/google/uuid v1.1.1
	github.com/googleapis/gnostic v0.2.0 // indirect
	github.com/grpc-ecosystem/grpc-gateway v1.8.5 // indirect
	github.com/mailru/easyjson v0.7.0 // indirect
	github.com/metal-pod/metal-go v0.2.0
	github.com/metal-pod/security v0.0.0-20190920091500-ed81ae92725b // indirect
	github.com/metal-pod/v v1.0.0
	github.com/pkg/errors v0.9.1
	github.com/prometheus/client_golang v0.9.4 // indirect
	github.com/soheilhy/cmux v0.1.4 // indirect
	github.com/spf13/pflag v1.0.5
	github.com/tmc/grpc-websocket-proxy v0.0.0-20190109142713-0ad062ec5ee5 // indirect
	github.com/xiang90/probing v0.0.0-20190116061207-43a291ad63a2 // indirect
	go.mongodb.org/mongo-driver v1.1.2 // indirect
	golang.org/x/net v0.0.0-20191101175033-0deb6923b6d9 // indirect
	k8s.io/api v0.16.6
	k8s.io/apiextensions-apiserver v0.16.6 // indirect
	k8s.io/apimachinery v0.16.6
	k8s.io/client-go v0.16.6
	k8s.io/cloud-provider v0.16.6
	k8s.io/component-base v0.16.6
	k8s.io/kubernetes v1.16.6
)

replace (
	k8s.io/api => k8s.io/api v0.16.6
	k8s.io/apiextensions-apiserver => k8s.io/apiextensions-apiserver v0.16.6
	k8s.io/apimachinery => k8s.io/apimachinery v0.16.6
	k8s.io/apiserver => k8s.io/apiserver v0.16.6
	k8s.io/cli-runtime => k8s.io/cli-runtime v0.16.6
	k8s.io/client-go => k8s.io/client-go v0.16.6
	k8s.io/cloud-provider => k8s.io/cloud-provider v0.16.6
	k8s.io/cluster-bootstrap => k8s.io/cluster-bootstrap v0.16.6
	k8s.io/code-generator => k8s.io/code-generator v0.16.6
	k8s.io/component-base => k8s.io/component-base v0.16.6
	k8s.io/cri-api => k8s.io/cri-api v0.16.6
	k8s.io/csi-translation-lib => k8s.io/csi-translation-lib v0.16.6
	k8s.io/kube-aggregator => k8s.io/kube-aggregator v0.16.6
	k8s.io/kube-controller-manager => k8s.io/kube-controller-manager v0.16.6
	k8s.io/kube-proxy => k8s.io/kube-proxy v0.16.6
	k8s.io/kube-scheduler => k8s.io/kube-scheduler v0.16.6
	k8s.io/kubectl => k8s.io/kubectl v0.16.6
	k8s.io/kubelet => k8s.io/kubelet v0.16.6
	k8s.io/legacy-cloud-providers => k8s.io/legacy-cloud-providers v0.16.6
	k8s.io/metrics => k8s.io/metrics v0.16.6
	k8s.io/sample-apiserver => k8s.io/sample-apiserver v0.16.6
)
