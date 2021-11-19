module github.com/metal-stack/metal-ccm

go 1.16

require (
	github.com/avast/retry-go/v3 v3.1.1
	github.com/google/go-cmp v0.5.6
	github.com/google/uuid v1.3.0
	github.com/metal-stack/metal-go v0.15.7
	github.com/metal-stack/metal-lib v0.8.2
	github.com/metal-stack/v v1.0.3
	github.com/onsi/ginkgo v1.16.4 // indirect
	github.com/spf13/pflag v1.0.5
	github.com/stretchr/testify v1.7.0
	k8s.io/api v0.22.2
	k8s.io/apimachinery v0.22.2
	k8s.io/client-go v0.22.2
	k8s.io/cloud-provider v0.22.2
	k8s.io/component-base v0.22.2
	k8s.io/klog/v2 v2.10.0
	k8s.io/utils v0.0.0-20210819203725-bdf08cb9a70a
	sigs.k8s.io/yaml v1.3.0
)

replace github.com/go-logr/logr => github.com/go-logr/logr v0.4.0
