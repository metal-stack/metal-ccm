/*
Copyright 2017 DigitalOcean

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

//nolint
package metal

import (
	"context"
	v1 "k8s.io/api/core/v1"
	"k8s.io/component-base/logs"
	"strings"

	//"context"
	"errors"
	"fmt"

	//metalgo "github.com/metal-pod/metal-go"
	"log"
	//"net/http"
	//"sort"
	//"strconv"
	//"strings"
	//v1 "k8s.io/api/core/v1"
	//cloudprovider "k8s.io/cloud-provider"
	//"k8s.io/klog"
)

const (
	// annoDOLoadBalancerID is the annotation specifying the load-balancer ID
	// used to enable fast retrievals of load-balancers from the API by UUID.
	annoDOLoadBalancerID = "kubernetes.digitalocean.com/load-balancer-id"

	// annDOProtocol is the annotation used to specify the default protocol
	// for DO load balancers. For ports specified in annDOTLSPorts, this protocol
	// is overwritten to https. Options are tcp, http and https. Defaults to tcp.
	annDOProtocol = "service.beta.kubernetes.io/do-loadBalancer-protocol"

	// annDOHealthCheckPath is the annotation used to specify the health check path
	// for DO load balancers. Defaults to '/'.
	annDOHealthCheckPath = "service.beta.kubernetes.io/do-loadBalancer-healthcheck-path"

	// annDOHealthCheckProtocol is the annotation used to specify the health check protocol
	// for DO load balancers. Defaults to the protocol used in
	// 'service.beta.kubernetes.io/do-loadBalancer-protocol'.
	annDOHealthCheckProtocol = "service.beta.kubernetes.io/do-loadBalancer-healthcheck-protocol"

	// annDOHealthCheckIntervalSeconds is the annotation used to specify the
	// number of seconds between between two consecutive health checks. The
	// value must be between 3 and 300. Defaults to 3.
	annDOHealthCheckIntervalSeconds = "service.beta.kubernetes.io/do-loadBalancer-healthcheck-check-interval-seconds"

	// annDOHealthCheckResponseTimeoutSeconds is the annotation used to specify the
	// number of seconds the Load Balancer instance will wait for a response
	// until marking a health check as failed. The value must be between 3 and
	// 300. Defaults to 5.
	annDOHealthCheckResponseTimeoutSeconds = "service.beta.kubernetes.io/do-loadBalancer-healthcheck-response-timeout-seconds"

	// annDOHealthCheckUnhealthyThreshold is the annotation used to specify the
	// number of times a health check must fail for a backend Droplet to be
	// marked "unhealthy" and be removed from the pool for the given service.
	// The value must be between 2 and 10. Defaults to 3.
	annDOHealthCheckUnhealthyThreshold = "service.beta.kubernetes.io/do-loadBalancer-healthcheck-unhealthy-threshold"

	// annDOHealthCheckHealthyThreshold is the annotation used to specify the
	// number of times a health check must pass for a backend Droplet to be
	// marked "healthy" for the given service and be re-added to the pool. The
	// value must be between 2 and 10. Defaults to 5.
	annDOHealthCheckHealthyThreshold = "service.beta.kubernetes.io/do-loadBalancer-healthcheck-healthy-threshold"

	// annDOTLSPorts is the annotation used to specify which ports of the load balancer
	// should use the HTTPS protocol. This is a comma separated list of ports
	// (e.g., 443,6443,7443).
	annDOTLSPorts = "service.beta.kubernetes.io/do-loadBalancer-tls-ports"

	// annDOHTTP2Ports is the annotation used to specify which ports of the load balancer
	// should use the HTTP2 protocol. This is a comma separated list of ports
	// (e.g., 443,6443,7443).
	annDOHTTP2Ports = "service.beta.kubernetes.io/do-loadBalancer-http2-ports"

	// annDOTLSPassThrough is the annotation used to specify whether the
	// DO loadBalancer should pass encrypted data to backend droplets.
	// This is optional and defaults to false.
	annDOTLSPassThrough = "service.beta.kubernetes.io/do-loadBalancer-tls-passthrough"

	// annDOCertificateID is the annotation specifying the certificate ID
	// used for https protocol. This annotation is required if annDOTLSPorts
	// is passed.
	annDOCertificateID = "service.beta.kubernetes.io/do-loadBalancer-certificate-id"

	// annDOHostname is the annotation specifying the hostname to use for the LB.
	annDOHostname = "service.beta.kubernetes.io/do-loadBalancer-hostname"

	// annDOAlgorithm is the annotation specifying which algorithm DO load balancer
	// should use. Options are round_robin and least_connections. Defaults
	// to round_robin.
	annDOAlgorithm = "service.beta.kubernetes.io/do-loadBalancer-algorithm"

	// annDOStickySessionsType is the annotation specifying which sticky session type
	// DO loadBalancer should use. Options are none and cookies. Defaults
	// to none.
	annDOStickySessionsType = "service.beta.kubernetes.io/do-loadBalancer-sticky-sessions-type"

	// annDOStickySessionsCookieName is the annotation specifying what cookie name to use for
	// DO loadBalancer sticky session. This annotation is required if
	// annDOStickySessionType is set to cookies.
	annDOStickySessionsCookieName = "service.beta.kubernetes.io/do-loadBalancer-sticky-sessions-cookie-name"

	// annDOStickySessionsCookieTTL is the annotation specifying TTL of cookie used for
	// DO load balancer sticky session. This annotation is required if
	// annDOStickySessionType is set to cookies.
	annDOStickySessionsCookieTTL = "service.beta.kubernetes.io/do-loadBalancer-sticky-sessions-cookie-ttl"

	// annDORedirectHTTPToHTTPS is the annotation specifying whether or not Http traffic
	// should be redirected to Https. Defaults to false
	annDORedirectHTTPToHTTPS = "service.beta.kubernetes.io/do-loadBalancer-redirect-http-to-https"

	// annDOEnableProxyProtocol is the annotation specifying whether PROXY protocol should
	// be enabled. Defaults to false.
	annDOEnableProxyProtocol = "service.beta.kubernetes.io/do-loadBalancer-enable-proxy-protocol"

	// defaultActiveTimeout is the number of seconds to wait for a load balancer to
	// reach the active state.
	defaultActiveTimeout = 90

	// defaultActiveCheckTick is the number of seconds between load balancer
	// status checks when waiting for activation.
	defaultActiveCheckTick = 5

	// statuses for Digital Ocean load balancer
	lbStatusNew     = "new"
	lbStatusActive  = "active"
	lbStatusErrored = "errored"

	// This is the DO-specific tag component prepended to the cluster ID.
	tagPrefixClusterID = "k8s"

	// Sticky sessions types.
	stickySessionsTypeNone    = "none"
	stickySessionsTypeCookies = "cookies"

	// Protocol values.
	protocolTCP   = "tcp"
	protocolHTTP  = "http"
	protocolHTTPS = "https"
	protocolHTTP2 = "http2"

	// Port protocol values.
	portProtocolTCP = "TCP"

	defaultSecurePort = 443
)

var errLBNotFound = errors.New("loadBalancer not found")

//func buildK8sTag(val string) string {
//	return fmt.Sprintf("%s:%s", tagPrefixClusterID, val)
//}

type loadBalancerController struct {
	resources *resources
	logger    *log.Logger
	resctl    *ResourcesController
}

type loadBalancer struct {
	ip string
}

// newLoadBalancer returns a cloudprovider.LoadBalancer whose concrete type is a *metal.loadBalancer.
func newLoadBalancer(resources *resources) *loadBalancerController {
	logs.InitLogs()
	logger := logs.NewLogger("metal-ccm loadbalancer ")

	return &loadBalancerController{
		resources: resources,
		logger:    logger,
	}
}

// GetLoadBalancer returns whether the specified load balancer exists, and if so, what its status is.
// Parameter 'clusterName' is the name of the cluster as presented to kube-controller-manager.
func (lbc *loadBalancerController) GetLoadBalancer(ctx context.Context, clusterName string, service *v1.Service) (status *v1.LoadBalancerStatus, exists bool, err error) {
	lbc.logger.Printf("GetLoadBalancer: ClusterName %q, Namespace %q, ServiceName %q\n", clusterName, service.Namespace, service.Name)
	//lb, err := lbc.retrieveAndAnnotateLoadBalancer(ctx, service)
	//if err != nil {
	//	if err == errLBNotFound {
	//		return nil, false, nil
	//	}
	//	return nil, false, err
	//}
	//
	//return &v1.LoadBalancerStatus{
	//	Ingress: []v1.LoadBalancerIngress{
	//		{
	//			IP: lb.ip,
	//		},
	//	},
	//}, true, nil
	return nil, false, nil
}

// GetLoadBalancerName returns the name of the load balancer.
func (lbc *loadBalancerController) GetLoadBalancerName(ctx context.Context, clusterName string, service *v1.Service) string {
	lbc.logger.Printf("GetLoadBalancerName: ClusterName %q, Namespace %q, ServiceName %q\n", clusterName, service.Namespace, service.Name)
	return fmt.Sprintf("lb-%s", clusterName)
}

// EnsureLoadBalancer ensures that the cluster is running a load balancer for service.
// It creates a new load balancer or updates the existing one. Returns the status of the balancer.
// Neither 'service' nor 'nodes' are modified.
// Parameter 'clusterName' is the name of the cluster as presented to kube-controller-manager.
func (lbc *loadBalancerController) EnsureLoadBalancer(ctx context.Context, clusterName string, service *v1.Service, nodes []*v1.Node) (*v1.LoadBalancerStatus, error) {
	var nn []string
	for _, n := range nodes {
		nn = append(nn, fmt.Sprintf("  - Cluster %q, Namespace %q, Name %q", n.ClusterName, n.Namespace, n.Name))
	}

	lbc.logger.Printf("EnsureLoadBalancer: ClusterName %q, Namespace %q, ServiceName %q, Nodes:\n%s\n", clusterName, service.Namespace, service.Name, strings.Join(nn, "\n"))

	//TODO lbc.restctl.AcquireIPs(project, network, count)

	err := lbc.resctl.SyncMetalLBConfig()
	if err != nil {
		return nil, err
	}

	lb := &loadBalancer{ //TODO
		ip: "10.100.0.1",
	}

	return &v1.LoadBalancerStatus{
		Ingress: []v1.LoadBalancerIngress{
			{
				IP: lb.ip,
			},
		},
	}, nil
}

// UpdateLoadBalancer updates hosts under the specified load balancer.
// Neither 'service' nor 'nodes' are modified.
// Parameter 'clusterName' is the name of the cluster as presented to kube-controller-manager.
func (lbc *loadBalancerController) UpdateLoadBalancer(ctx context.Context, clusterName string, service *v1.Service, nodes []*v1.Node) error {
	var nn []string
	for _, n := range nodes {
		nn = append(nn, fmt.Sprintf("  - Cluster %q, Namespace %q, Name %q", n.ClusterName, n.Namespace, n.Name))
	}
	lbc.logger.Printf("UpdateLoadBalancer: ClusterName %q, Namespace %q, ServiceName %q, Nodes:\n%s\n", clusterName, service.Namespace, service.Name, strings.Join(nn, "\n"))

	err := lbc.resctl.SyncMetalLBConfig()
	if err != nil {
		return err
	}

	return nil
}

// EnsureLoadBalancerDeleted deletes the cluster load balancer if it
// exists, returning nil if the load balancer specified either didn't exist or
// was successfully deleted.
// This construction is useful because many cloud providers' load balancers
// have multiple underlying components, meaning a Get could say that the LB
// doesn't exist even if some part of it is still laying around.
// Parameter 'service' is not modified.
// Parameter 'clusterName' is the name of the cluster as presented to kube-controller-manager
func (lbc *loadBalancerController) EnsureLoadBalancerDeleted(ctx context.Context, clusterName string, service *v1.Service) error {
	lbc.logger.Printf("EnsureLoadBalancerDeleted: ClusterName %q, Namespace %q, ServiceName %q\n", clusterName, service.Namespace, service.Name)
	return nil
}
