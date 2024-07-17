package cluster2

import (
	"context"
	"errors"
	"github.com/Pallinder/go-randomdata"
	"github.com/hetznercloud/hcloud-go/v2/hcloud"
	"github.com/juju/juju/cloudconfig/cloudinit"
	"github.com/juju/packaging/v3"
	"github.com/xetys/hetzner-kube/pkg/clustermanager"
	"github.com/xetys/hetzner-kube/pkg/hetzner"
	"github.com/xetys/hetzner-kube/ssh"
	"github.com/xetys/hetzner-kube/types"
	"io"
	"net"
	"net/http"
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

	//TODO Ping provider
	hetznerProvider := hetzner.NewHetznerProvider2(ctx, apiToken, config.Metadata.Name)

	_, ipnet, err := net.ParseCIDR("10.0.0.0/16")
	network, err := hetznerProvider.CreateNetwork(ctx, config.Metadata.Name, ipnet) //TODO FIXME Hardcoded
	if err != nil {
		return nil, err
	}

	_, err = hetznerProvider.CreateFirewall(ctx, config.Metadata.Name, []string{"22", "6443"})
	if err != nil {
		return nil, err
	}

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
	var osName string = "ubuntu"
	if strings.Contains(strings.ToLower(config.Spec.Nodes.Master.Image), "ubuntu") {
		osName = "ubuntu"
	} else if strings.Contains(strings.ToLower(config.Spec.Nodes.Master.Image), "centos") {
		osName = "centos"
	} else {
		return nil, errors.New("unsupported node image type")
	}
	_ = osName
	config.Spec.Nodes.Master.CloudInit, err = cloudinit.New(osName)
	if err != nil {
		return nil, err
	}

	err = masterCloudInit(config.Spec.Nodes.Master.CloudInit)
	if err != nil {
		return nil, err
	}

	config.Spec.Nodes.Master.CloudInit.SetSystemUpdate(true)
	config.Spec.Nodes.Master.CloudInit.SetSystemUpgrade(true)

	config.Spec.Nodes.Master.Networks = []*hcloud.Network{network}
	var templates []types.NodeConfig

	for range config.Spec.Nodes.Master.Replicas {
		templates = append(templates, config.Spec.Nodes.Master)
	}

	config.Spec.Nodes.Worker.CloudInit = config.Spec.Nodes.Master.CloudInit

	config.Spec.Nodes.Worker.Labels = make(map[string]string)
	config.Spec.Nodes.Master.Labels[types.ClusterRoleLabel] = string(types.WorkerNodeRole)

	config.Spec.Nodes.Worker.Networks = []*hcloud.Network{network}

	for range config.Spec.Nodes.Worker.Replicas {
		//templates = append(templates, config.Spec.Nodes.Worker)
	}

	//config.Spec.Nodes.Etcd.Labels = make(map[string]string)

	var nodes []clustermanager.Node
	for _, template := range templates {
		newNode, err := newCluster.createNode(ctx, template)
		if err != nil {
			println(err.Error())
			continue
		}

		nodes = append(nodes, newNode)
	}

	return &newCluster, nil
}

func masterCloudInit(ci cloudinit.CloudConfig) (err error) {
	resK8S, err := http.Get("https://pkgs.k8s.io/core:/stable:/v1.30/deb/Release.key")
	if err != nil {
		return err
	}
	defer resK8S.Body.Close()

	k8sKey, err := io.ReadAll(resK8S.Body)
	if err != nil {
		return err
	}

	if !strings.Contains(string(k8sKey), "BEGIN PGP PUBLIC KEY BLOCK") || !strings.Contains(string(k8sKey), "END PGP PUBLIC KEY BLOCK") {
		return errors.New("malformed gpg key for k8s repo")
	}

	//FIXME hardcoded k8s version
	ci.AddPackageSource(packaging.PackageSource{
		Name: "K8S Repo",
		URL:  "deb [signed-by=$KEY_FILE] https://pkgs.k8s.io/core:/stable:/v1.30/deb/ /",
		Key:  string(k8sKey),
	})

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

	ci.AddRunCmd(`swapoff -a`)
	ci.AddRunCmd(`sed -i '/ swap / s/^\(.*\)$/#\1/g' /etc/fstab`)

	ci.AddRunCmd(`apt-mark hold kubelet kubeadm kubectl`)

	ci.AddRunCmd(`systemctl enable --now kubelet`)

	//ci.AddRunCmd(`kubeadm init`)
	//
	//ci.AddRunCmd(`mkdir -p $HOME/.kube`)
	//ci.AddRunCmd(`sudo cp -i /etc/kubernetes/admin.conf $HOME/.kube/config`)
	//ci.AddRunCmd(`sudo chown $(id -u):$(id -g) $HOME/.kube/config`)
	//
	//ci.AddRunCmd(`kubectl apply -f https://raw.githubusercontent.com/coreos/flannel/master/Documentation/kube-flannel.yml`)

	return
}
