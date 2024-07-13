package phases

import (
	"errors"
	"github.com/xetys/hetzner-kube/pkg/clustermanager"
	"log/slog"
)

// ProvisionNodesPhase defines the phase which install all the tools for each node
type ProvisionNodesPhase struct {
	clusterManager *clustermanager.Manager
}

// NewProvisionNodesPhase returns an instance of *ProvisionNodesPhase
func NewProvisionNodesPhase(manager *clustermanager.Manager) Phase {
	return &ProvisionNodesPhase{
		clusterManager: manager,
	}
}

// ShouldRun returns if this phase should run
func (phase *ProvisionNodesPhase) ShouldRun() bool {
	return true
}

// Run runs the phase
func (phase *ProvisionNodesPhase) Run() error {
	cluster := phase.clusterManager.Cluster()

	for i := 0; i < 3; i++ {
		err := phase.clusterManager.ProvisionNodes(cluster.Nodes)
		if err != nil {
			slog.Error(err.Error())
			continue
		}

		return nil
	}

	return errors.New("max tries count (3) excited on NodeProvisioning")
}
