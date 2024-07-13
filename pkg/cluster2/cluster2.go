package cluster2

import (
	"context"
	"errors"
	"fmt"
	"github.com/Pallinder/go-randomdata"
	"github.com/xetys/hetzner-kube/pkg/clustermanager"
	"github.com/xetys/hetzner-kube/pkg/hetzner"
	"github.com/xetys/hetzner-kube/types"
	"log/slog"
	"time"
)

type Cluster struct {
	Config   types.ClusterConfig
	provider *hetzner.Provider
}

func CreateNewCluster(ctx context.Context, config types.ClusterConfig) (*Cluster, error) {
	if config.ClusterName == "" {
		config.ClusterName = randomdata.FirstName(0)
	}

	if config.MasterNode.MasterNodeTemplate.Labels == nil {
		config.MasterNode.MasterNodeTemplate.Labels = make(map[string]string)
	}

	hetznerProvider := hetzner.NewHetznerProvider2(ctx, config.HetznerApiKey, config.SSHKey.Name)
	//TODO Ping provider

	newCluster := Cluster{
		Config:   config,
		provider: hetznerProvider,
	}

	//masterNode, err := newCluster.createNode(ctx, config.MasterNode.MasterNodeTemplate)
	//if err != nil {
	//	return nil, err
	//}
	//
	//_ = masterNode

	return &newCluster, nil
}

func (c *Cluster) createNode(ctx context.Context, nodeTemplate clustermanager.NodeTemplate) (clustermanager.Node, error) {
	nodeTemplate.Labels[types.ClusterNameLabel] = c.Config.ClusterName
	nodeTemplate.Labels[types.ClusterRoleLabel] = string(types.MasterNodeRole)

	node, err := c.provider.CreateNode2(ctx, nodeTemplate)
	if err != nil {
		return clustermanager.Node{}, err
	}

	sshClient := clustermanager.NewSSHCommunicator([]clustermanager.SSHKey{c.Config.SSHKey}, false)
	err = sshClient.(*clustermanager.SSHCommunicator).CapturePassphrase(c.Config.SSHKey.Name)

	slog.Info("wait for cloud-init completion with deadline 300s")
	const tick = time.Second * 10
	ctxDeadline, cancelFunc := context.WithTimeout(ctx, time.Minute*5)
	defer cancelFunc()

	var counter int
	var t time.Time
	var isOk bool
	for {
		if t, isOk = ctxDeadline.Deadline(); !isOk {
			return clustermanager.Node{}, errors.New("cloud init deadline excited")
		}

		_, err := sshClient.RunCmd(node, "[ -f /tmp/hetzner-kube.unlock ]")
		if err == nil {
			println()
			slog.Info(fmt.Sprintf("cloud init complete with time: %d seconds", t.Second()))
			break
		}

		counter++
		print(fmt.Sprintf("...%ds", int(tick.Seconds())*counter))
		time.Sleep(tick)
	}

	_, err = sshClient.RunCmd(node, "rm /tmp/hetzner-kube.unlock")
	if err != nil {
		slog.Warn("can't delete cloud-init lock file")
	}

	return clustermanager.Node{}, nil
}
