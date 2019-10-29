package constants

const (
	MetalAPIUrlEnvVar      = "METAL_API_URL"
	MetalAuthTokenEnvVar   = "METAL_AUTH_TOKEN"
	MetalAuthHMACEnvVar    = "METAL_AUTH_HMAC"
	MetalProjectIDEnvVar   = "METAL_PROJECT_ID"
	MetalPartitionIDEnvVar = "METAL_PARTITION_ID"
	MetalClusterIDEnvVar   = "METAL_CLUSTER_ID"

	ProviderName = "metal"

	ASNNodeLabel = "machine.metal-pod.io/network.primary.asn"

	CalicoIPv4IPIPTunnelAddr  = "projectcalico.org/IPv4IPIPTunnelAddr"
	CalicoIPv4VXLANTunnelAddr = "projectcalico.org/IPv4VXLANTunnelAddr"

	MetalLBSpecificAddressPool = "metallb.universe.tf/address-pool"

	TagClusterPrefix = "cluster.metal-pod.io"
	TagServicePrefix = "service.metal-pod.io/clusterid/namespace/servicename"
	TagMachinePrefix = "machine.metal-pod.io"

	IPPrefix = "metallb-"
)

var (
	CalicoAnnotations = []string{
		CalicoIPv4IPIPTunnelAddr,
		CalicoIPv4VXLANTunnelAddr,
	}
)
