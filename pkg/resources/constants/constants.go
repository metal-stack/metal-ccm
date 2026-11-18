package constants

const (
	MetalAPIUrlEnvVar = "METAL_API_URL"
	//nolint
	MetalAuthTokenEnvVar              = "METAL_AUTH_TOKEN"
	MetalAuthHMACEnvVar               = "METAL_AUTH_HMAC"
	MetalAuthHMACAuthTypeEnvVar       = "METAL_AUTH_HMAC_AUTH_TYPE"
	MetalProjectIDEnvVar              = "METAL_PROJECT_ID"
	MetalPartitionIDEnvVar            = "METAL_PARTITION_ID"
	MetalClusterIDEnvVar              = "METAL_CLUSTER_ID"
	MetalDefaultExternalNetworkEnvVar = "METAL_DEFAULT_EXTERNAL_NETWORK_ID"
	MetalAdditionalNetworks           = "METAL_ADDITIONAL_NETWORKS"

	// MetalSSHPublicKey latest ssh public key
	MetalSSHPublicKey = "METAL_SSH_PUBLICKEY"

	ProviderName = "metal"

	// MetalLBAddressPoolAnnotation is the metallb annotation used to select the address pool from
	// which the load balancer IP is acquired.
	MetalLBAddressPoolAnnotation = "metallb.io/address-pool"

	// Deprecated: use MetalLBAddressPoolAnnotation instead. Kept for backwards compatibility.
	LegacyMetalLBAddressPoolAnnotation = "metallb.universe.tf/address-pool"

	// MetalLBLoadBalancerIPs is the metallb annotation used to pin the load balancer IPs to a
	// service. It may contain multiple comma-separated IP addresses and replaces the deprecated
	// spec.loadBalancerIP field.
	MetalLBLoadBalancerIPs = "metallb.io/loadBalancerIPs"

	IPPrefix = "metallb-"

	Loadbalancer = "LOADBALANCER"
)
