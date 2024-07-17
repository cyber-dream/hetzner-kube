package types

import (
	"github.com/google/uuid"
	"github.com/hetznercloud/hcloud-go/v2/hcloud"
	"github.com/juju/juju/cloudconfig/cloudinit"
	"github.com/xetys/hetzner-kube/pkg/clustermanager"
)

type ClusterConfigOld struct {
	uuid          uuid.UUID             `yaml:"-"`
	MasterNode    MasterNodeConfig      `yaml:"masterNode"`
	ClusterName   string                `yaml:"clusterName"`
	SSHKey        clustermanager.SSHKey `yaml:"SSHKey"`
	HetznerApiKey string                `yaml:"hetznerApiKey"`
}

func (c *ClusterConfigOld) GetUUID() uuid.UUID {
	if c.uuid == uuid.Nil {
		c.uuid = uuid.New()
	}
	return c.uuid
}

type MasterNodeConfig struct {
	MasterNodesCount   uint8                       `yaml:"masterNodesCount"`
	MasterNodeTemplate clustermanager.NodeTemplate `yaml:"-"`
	//MasterNodeCloudInitPath string `yaml:"masterNodeCloudInitPath"`
}

type ClusterConfig struct {
	Kind     string `yaml:"kind"`
	Metadata struct {
		Name string `yaml:"name"`
	} `yaml:"metadata"`
	NodesControl struct {
		//SSHKeyName string `yaml:"ssh-key-name"`
	} `yaml:"nodes-control"`
	Spec struct {
		Nodes struct {
			Master NodeConfig  `yaml:"master"`
			Worker NodeConfig  `yaml:"worker"`
			Etcd   *NodeConfig `yaml:"etcd,omitempty"`
		} `yaml:"nodes"`
	} `yaml:"spec"`
}

type NodeConfig struct {
	Replicas    int                   `yaml:"replicas"`
	Type        string                `yaml:"type"`
	Image       string                `yaml:"image"`
	DataCenters []string              `yaml:"dataCenters"`
	Labels      map[string]string     `yaml:"-"`
	CloudInit   cloudinit.CloudConfig `yaml:"-"`
	Networks    []*hcloud.Network     `yaml:"-"`
}
