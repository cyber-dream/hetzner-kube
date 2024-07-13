package types

import (
	"github.com/google/uuid"
	"github.com/xetys/hetzner-kube/pkg/clustermanager"
)

type ClusterConfig struct {
	uuid          uuid.UUID             `yaml:"-"`
	MasterNode    MasterNodeConfig      `yaml:"masterNode"`
	ClusterName   string                `yaml:"clusterName"`
	SSHKey        clustermanager.SSHKey `yaml:"SSHKey"`
	HetznerApiKey string                `yaml:"hetznerApiKey"`
}

func (c *ClusterConfig) GetUUID() uuid.UUID {
	if c.uuid == uuid.Nil {
		c.uuid = uuid.New()
	}
	return c.uuid
}

type MasterNodeConfig struct {
	MasterNodesCount uint8 `yaml:"masterNodesCount"`
	//MasterNodeTemplate      clustermanager.NodeTemplate `yaml:"-"`
	//MasterNodeCloudInitPath string `yaml:"masterNodeCloudInitPath"`
}
