package types

import (
	"errors"
	"fmt"
	"github.com/google/uuid"
	"github.com/hetznercloud/hcloud-go/v2/hcloud"
	"github.com/juju/juju/cloudconfig/cloudinit"
	"github.com/juju/packaging/v3"
	"github.com/xetys/hetzner-kube/pkg/clustermanager"
	"io"
	"net/http"
	"strings"
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
		Network struct {
			HetznerCidr string `yaml:"hetzner-cidr"`
			FlannelCidr string `yaml:"flannel-cidr"`
		} `yaml:"network"`
		Nodes struct {
			Master ProviderNodeConfig  `yaml:"master"`
			Worker ProviderNodeConfig  `yaml:"worker"`
			Etcd   *ProviderNodeConfig `yaml:"etcd,omitempty"`
		} `yaml:"nodes"`
	} `yaml:"spec"`
}

type ProviderNodeConfig struct {
	Replicas    int      `yaml:"replicas"`
	Type        string   `yaml:"type"`
	Image       string   `yaml:"image"`
	DataCenters []string `yaml:"dataCenters"`
}

// JuJuOsName
// TODO Maybe move to provider realisation. They have static list of os images
func (c ProviderNodeConfig) JuJuOsName() (string, error) {
	switch true {
	//Debian???
	case strings.Contains(strings.ToLower(c.Image), "ubuntu"):
		return "ubuntu", nil
	case strings.Contains(strings.ToLower(c.Image), "fedora"):
	case strings.Contains(strings.ToLower(c.Image), "centos"):
		return "centos", nil
	}

	return "", errors.New("unsupported node image type")
}

type ProviderNodeTemplate struct {
	ProviderNodeConfig
	// Name of virtual machine in provider cloud
	Name       string
	Role       NodeRole
	Labels     map[string]string
	CloudInit  cloudinit.CloudConfig
	Networks   []*hcloud.Network
	Commands   []string
	SSHKeyName string
}

func NewProviderNodeTemplate(clusterName string, inConfig ProviderNodeConfig, inRole NodeRole) (ProviderNodeTemplate, error) {
	osName, err := inConfig.JuJuOsName()
	if err != nil {
		return ProviderNodeTemplate{}, err
	}

	ci, err := cloudinit.New(osName)
	if err != nil {
		return ProviderNodeTemplate{}, err
	}

	//FIXME Hardcoded K8S version
	resK8S, err := http.Get("https://pkgs.k8s.io/core:/stable:/v1.30/deb/Release.key")
	if err != nil {
		return ProviderNodeTemplate{}, err
	}
	defer resK8S.Body.Close()

	k8sKey, err := io.ReadAll(resK8S.Body)
	if err != nil {
		return ProviderNodeTemplate{}, err
	}

	if !strings.Contains(string(k8sKey), "BEGIN PGP PUBLIC KEY BLOCK") || !strings.Contains(string(k8sKey), "END PGP PUBLIC KEY BLOCK") {
		return ProviderNodeTemplate{}, errors.New("malformed gpg key for k8s repoK8S")
	}
	//FIXME hardcoded k8s version
	repoK8S, err := getSignedRepo(
		"K8S Repo",
		"https://pkgs.k8s.io/core:/stable:/v1.30/deb/Release.key",
		"deb [signed-by=$KEY_FILE] https://pkgs.k8s.io/core:/stable:/v1.30/deb/ /")
	if err != nil {
		return ProviderNodeTemplate{}, err
	}
	ci.AddPackageSource(*repoK8S)

	ci.SetSystemUpdate(true)
	ci.SetSystemUpgrade(true)

	ci.AddPackage("apt-transport-https")
	ci.AddPackage("ca-certificates")
	ci.AddPackage("curl")
	ci.AddPackage("gpg")
	ci.AddPackage("docker.io")
	ci.AddPackage("kubelet")
	ci.AddPackage("kubeadm")
	ci.AddPackage("kubectl")

	ci.AddRunCmd(`apt-mark hold kubelet kubeadm kubectl`)

	ci.AddRunCmd(`swapoff -a`)
	//ci.AddRunCmd(`sed -i '/ swap / s/^\(.*\)$/#\1/g' /etc/fstab`)
	ci.AddRunCmd(`modprobe overlay -v`)
	ci.AddRunCmd(`modprobe br_netfilter -v`)

	ci.AddRunCmd(`echo "overlay" >> /etc/modules`)
	ci.AddRunCmd(`echo "br_netfilter" >> /etc/modules`)

	ci.AddRunCmd(`echo 1 > /proc/sys/net/ipv4/ip_forward`)

	//ci.AddRunCmd(`sudo ufw disable`)
	//ci.AddRunCmd(`service apparmor stop`)
	//ci.AddRunCmd(`service apparmor teardown`)
	//ci.AddRunCmd(`/usr/sbin/update-rc.d -f apparmor remove`)

	//ci.AddRunCmd(`echo "net.ipv4.ip_forward = 1" | sudo tee -a /etc/sysctl.conf`)
	ci.AddRunCmd(`sudo sysctl -p`)

	return ProviderNodeTemplate{
		ProviderNodeConfig: inConfig,
		Labels:             make(map[string]string),
		CloudInit:          ci,
		Networks:           nil,
		Commands:           nil,
		Name:               fmt.Sprintf("%s-%s", clusterName, inRole),
		Role:               inRole,
	}, nil
}

func getSignedRepo(inName string, inGPGUrl string, inSourceStr string) (*packaging.PackageSource, error) {
	resGPG, err := http.Get(inGPGUrl)
	if err != nil {
		return nil, err
	}
	defer resGPG.Body.Close()

	key, err := io.ReadAll(resGPG.Body)
	if err != nil {
		return nil, err
	}

	if !strings.Contains(string(key), "BEGIN PGP PUBLIC KEY BLOCK") || !strings.Contains(string(key), "END PGP PUBLIC KEY BLOCK") {
		return nil, errors.New("malformed gpg key for k8s repo")
	}

	return &packaging.PackageSource{
		Name: inName,
		URL:  inSourceStr,
		Key:  string(key),
	}, nil
}
