package constants

const (
	MetalAPIUrlEnvVar   = "METAL_API_URL"
	MetalAPITokenEnvVar = "METAL_API_TOKEN"

	MetalProjectIDEnvVar              = "METAL_PROJECT_ID"
	MetalPartitionIDEnvVar            = "METAL_PARTITION_ID"
	MetalClusterIDEnvVar              = "METAL_CLUSTER_ID"
	MetalDefaultExternalNetworkEnvVar = "METAL_DEFAULT_EXTERNAL_NETWORK_ID"
	MetalAdditionalNetworks           = "METAL_ADDITIONAL_NETWORKS"

	// MetalSSHPublicKey latest ssh public key
	MetalSSHPublicKey = "METAL_SSH_PUBLICKEY"

	ProviderName = "metal"

	// FIXME this annotation is deprecated metallb.io should be used instead
	MetalLBSpecificAddressPool = "metallb.universe.tf/address-pool"

	IPPrefix = "metallb-"

	Loadbalancer = "LOADBALANCER"
)
