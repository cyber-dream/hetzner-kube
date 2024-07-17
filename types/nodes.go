package types

const (
	//Hetzner nodes labels
	ClusterNameLabel             = "cluster-name"
	ClusterRoleLabel             = "cluster-role"
	ClusterHighAvailabilityLabel = "high-availability"
	ClusterIsolatedEtcdLabel     = "isolated-etcd"
	ClusterCIDRLabel             = "cidr"

	// Hetzner nodes roles
	MasterNodeRole NodeRole = "master"
	WorkerNodeRole NodeRole = "worker"
	EtcdNodeRole   NodeRole = "etcd"
	AnyNodeRole    NodeRole = "any"
)

type NodeRole string

func (nr NodeRole) String() string {
	return string(nr)
}
