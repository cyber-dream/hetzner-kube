package cluster2

import (
	"context"
	"fmt"
	"github.com/Pallinder/go-randomdata"
	"github.com/hetznercloud/hcloud-go/v2/hcloud"
	"github.com/xetys/hetzner-kube/pkg/clustermanager"
	"github.com/xetys/hetzner-kube/pkg/hetzner"
	"github.com/xetys/hetzner-kube/ssh"
	"github.com/xetys/hetzner-kube/types"
	"net"
	"path/filepath"
)


func CreateNewCluster(ctx context.Context, config types.ClusterConfig) (*Cluster, error) {
	if config.Metadata.Name == "" {
		config.Metadata.Name = randomdata.FirstName(0)
	}

	//TODO Ping provider
	hetznerProvider := hetzner.NewHetznerProvider2(ctx, apiToken, config.Metadata.Name)

	// Create cluster stuff on Hetzner
	_, ipNet, err := net.ParseCIDR(config.Spec.Network.Cidr)
	//_, ipNet, err := net.ParseCIDR("10.244.0.0/16") //FIXME Hardcoded. Default cidr in Flannel, should edit flannel.yaml to override or edit .env on nodes
	if err != nil {
		return nil, err
	}

	network, err := hetznerProvider.CreateNetwork(ctx, config.Metadata.Name, ipNet)
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

	// Create Master Nodes

	masterTemplate, err := types.NewProviderNodeTemplate(config.Metadata.Name, config.Spec.Nodes.Master, types.MasterNodeRole)
	if err != nil {
		return nil, err
	}

	masterTemplate.Labels[types.ClusterNameLabel] = config.Metadata.Name
	masterTemplate.Labels[types.ClusterRoleLabel] = string(types.MasterNodeRole)

	//err = masterCloudInit(masterTemplate.CloudInit)
	//if err != nil {
	//	return nil, err
	//}

	//Post-init ssh commands
	//masterTemplate.Commands = []string{
	//	fmt.Sprintf(`kubeadm init --pod-network-cidr=%s --apiserver-advertise-address=%s`, ipNet.String()),
	//	`mkdir -p $HOME/.kube`,
	//	`sudo cp -i /etc/kubernetes/admin.conf $HOME/.kube/config`,
	//	`sudo chown $(id -u):$(id -g) $HOME/.kube/config`,
	//	`kubectl apply -f https://raw.githubusercontent.com/coreos/flannel/master/Documentation/kube-flannel.yml`,
	//}

	// Networks
	masterTemplate.Networks = []*hcloud.Network{network}
	//var templates []types.ProviderNodeTemplate

	//for range config.Spec.Nodes.Master.Replicas {
	//	templates = append(templates, masterTemplate)
	//}

	// Create Worker Nodes

	workerTemplate, err := types.NewProviderNodeTemplate(config.Metadata.Name, config.Spec.Nodes.Worker, types.WorkerNodeRole)
	if err != nil {
		return nil, err
	}

	workerTemplate.Labels[types.ClusterNameLabel] = config.Metadata.Name
	workerTemplate.Labels[types.ClusterRoleLabel] = string(types.WorkerNodeRole)

	workerTemplate.Networks = []*hcloud.Network{network}

	var masterNodes []clustermanager.Node
	for range config.Spec.Nodes.Master.Replicas {
		newNode, err := newCluster.createNode(ctx, masterTemplate)
		if err != nil {
			println(err.Error())
			continue
		}

		masterNodes = append(masterNodes, newNode)
	}

	sshClient := clustermanager.NewSSHCommunicator([]clustermanager.SSHKey{
		{
			Name:           config.Metadata.Name,
			PrivateKeyPath: filepath.Join("./.ssh/", config.Metadata.Name), //FIXME Hardcoded
			PublicKeyPath:  filepath.Join("./.ssh/", fmt.Sprintf("%s.pub", config.Metadata.Name)),
		},
	}, false)

	err = sshClient.(*clustermanager.SSHCommunicator).CapturePassphrase(config.Metadata.Name)
	if err != nil {
		return nil, err
	}

	for _, node := range masterNodes {

		commands := []string{
			fmt.Sprintf(`kubeadm init --pod-network-cidr=%s --apiserver-advertise-address=%s`, ipNet.String(), node.PrivateIPAddress),
			`mkdir -p $HOME/.kube`,
			`sudo cp -i /etc/kubernetes/admin.conf $HOME/.kube/config`,
			`sudo chown $(id -u):$(id -g) $HOME/.kube/config`,
			//`kubectl apply -f https://raw.githubusercontent.com/coreos/flannel/master/Documentation/kube-flannel.yml`,
			`kubectl create ns kube-flannel`,
			`kubectl label --overwrite ns kube-flannel pod-security.kubernetes.io/enforce=privileged`,
			`helm repo add flannel https://flannel-io.github.io/flannel/`,
			fmt.Sprintf(`helm install flannel --set podCidr="%s" --namespace kube-flannel flannel/flannel`, ipNet.String()),
		}

		for _, command := range commands {
			fmt.Printf("executing command: %s", command)

			data, err := sshClient.RunCmd(node, command)
			if err != nil {
				println(err.Error())
				continue
			}

			println(data)
		}
	}

	var workersNodes []clustermanager.Node
	for range config.Spec.Nodes.Worker.Replicas {
		newNode, err := newCluster.createNode(ctx, workerTemplate)
		if err != nil {
			println(err.Error())
			continue
		}

		workersNodes = append(workersNodes, newNode)
	}

	for _, node := range workersNodes {
		data, err := sshClient.RunCmd(masterNodes[0], `kubeadm token create --print-join-command`)
		if err != nil {
			println(err.Error())
			continue
		}

		commands := []string{
			data,
		}

		for _, command := range commands {
			fmt.Printf("executing command: %s", command)

			data, err := sshClient.RunCmd(node, command)
			if err != nil {
				println(err.Error())
				continue
			}

			if data != "" {
				println(data)
			}
		}
	}

	return &newCluster, nil
}
