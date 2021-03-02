module github.com/metal-stack/metal-ccm

go 1.15

require (
	github.com/NYTimes/gziphandler v1.0.1 // indirect
	github.com/ghodss/yaml v1.0.0
	github.com/google/uuid v1.2.0
	github.com/metal-stack/metal-go v0.12.2
	github.com/metal-stack/metal-lib v0.6.9
	github.com/metal-stack/v v1.0.3
	github.com/pkg/errors v0.9.1
	github.com/spf13/pflag v1.0.5
	k8s.io/api v0.20.4
	k8s.io/apimachinery v0.20.4
	k8s.io/client-go v0.20.4
	k8s.io/cloud-provider v0.20.4
	k8s.io/component-base v0.20.4
	k8s.io/klog/v2 v2.5.0
)

// specify a lower transitive dependency to grpc otherwise
// endpoint.go:114:78: undefined: resolver.BuildOption
replace google.golang.org/grpc => google.golang.org/grpc v1.26.0
