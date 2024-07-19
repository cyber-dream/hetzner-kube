package cluster2

import (
	"github.com/xetys/hetzner-kube/pkg/hetzner"
	"github.com/xetys/hetzner-kube/types"
)

type Cluster struct {
	Config   types.ClusterConfig
	provider *hetzner.Provider
}
