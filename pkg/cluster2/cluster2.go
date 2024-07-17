package cluster2

import (
	"context"
	"errors"
	"github.com/Pallinder/go-randomdata"
	"github.com/juju/juju/cloudconfig/cloudinit"
	"github.com/xetys/hetzner-kube/pkg/hetzner"
	"github.com/xetys/hetzner-kube/ssh"
	"github.com/xetys/hetzner-kube/types"
	"strings"
)

type Cluster struct {
	Config   types.ClusterConfig
	provider *hetzner.Provider
}

func CreateNewCluster(ctx context.Context, config types.ClusterConfig) (*Cluster, error) {
	if config.Metadata.Name == "" {
		config.Metadata.Name = randomdata.FirstName(0)
	}

	//if config.Spec.Nodes.Master.Metadata.Labels == nil {
	//	config.MasterNode.MasterNodeTemplate.Labels = make(map[string]string)
	//}

	//TODO Ping provider
	hetznerProvider := hetzner.NewHetznerProvider2(ctx, apiToken, config.Metadata.Name)

	publicKey, _, err := ssh.CreateSSHKey("./.ssh/", config.Metadata.Name)
	if err != nil {
		return nil, err
	}

	err = hetznerProvider.AddSSHKey(ctx, config.Metadata.Name, publicKey)
	if err != nil {
		return nil, err
	}

	newCluster := Cluster{
		Config:   config,
		provider: hetznerProvider,
	}

	config.Spec.Nodes.Master.Labels = make(map[string]string)
	config.Spec.Nodes.Master.Labels[types.ClusterRoleLabel] = string(types.MasterNodeRole)

	//TODO more detections
	var osName string
	if strings.Contains(strings.ToLower(config.Spec.Nodes.Master.Image), "ubuntu") {
		osName = "ubuntu"
	} else if strings.Contains(strings.ToLower(config.Spec.Nodes.Master.Image), "centos") {
		osName = "centos"
	} else {
		return nil, errors.New("unsupported node image type")
	}
	config.Spec.Nodes.Master.CloudInit, err = cloudinit.New(osName)
	if err != nil {
		return nil, err
	}

	config.Spec.Nodes.Master.CloudInit.SetSystemUpdate(true)
	config.Spec.Nodes.Master.CloudInit.SetSystemUpgrade(true)

	config.Spec.Nodes.Master.CloudInit.AddPackage("apt-transport-https")
	config.Spec.Nodes.Master.CloudInit.AddPackage("ca-certificates")
	config.Spec.Nodes.Master.CloudInit.AddPackage("curl")
	config.Spec.Nodes.Master.CloudInit.AddPackage("gnupg")
	config.Spec.Nodes.Master.CloudInit.AddPackage("lsb-release")
	config.Spec.Nodes.Master.CloudInit.AddPackage("wireguard-tools")

	masterNode, err := newCluster.createNode(ctx, config.Spec.Nodes.Master)
	if err != nil {
		return nil, err
	}

	_ = masterNode

	config.Spec.Nodes.Worker.Labels = make(map[string]string)
	//config.Spec.Nodes.Etcd.Labels = make(map[string]string)

	return &newCluster, nil
}
