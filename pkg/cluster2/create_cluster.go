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
	//_, ipNet, err := net.ParseCIDR(config.Spec.Network.Cidr)
	//_, ipNet, err := net.ParseCIDR("10.244.0.0/16") //FIXME Hardcoded. Default cidr in Flannel, should edit flannel.yaml to override or edit .env on nodes
	//if err != nil {
	//	return nil, err
	//}

	_, hetznerIpNet, err := net.ParseCIDR(config.Spec.Network.HetznerCidr)
	if err != nil {
		return nil, err
	}

	network, err := hetznerProvider.CreateNetwork(ctx, config.Metadata.Name, hetznerIpNet)
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

	// Networks
	masterTemplate.Networks = []*hcloud.Network{network}

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

	//maskInt, _ := ipNet.Mask.Size()
	//hosts, _, err := getAllHosts(ip, *ipNet)
	//if err != nil {
	//	return nil, err
	//}

	//flannelNetConf, err := json.Marshal(types.FlannelNetConf{
	//	Network:   ipNet.IP.String(),
	//	SubnetLen: maskInt,
	//	SubnetMin: hosts[1], //hosts[0] used by hetzner router
	//	SubnetMax: hosts[len(hosts)-1],
	//	Backend: types.FlannelBackendConf{
	//		Type: "udp",
	//		Port: 7890,
	//	},
	//})

	//	flannelRunConf := fmt.Sprintf(
	//		`FLANNEL_NETWORK=%s
	//FLANNEL_SUBNET=%s
	//FLANNEL_MTU=1450
	//FLANNEL_IPMASQ=true`, ipNet.String(), ipNet.String())

	_, flannelCidr, err := net.ParseCIDR(config.Spec.Network.HetznerCidr)
	if err != nil {
		return nil, err
	}

	for _, node := range masterNodes {
		//TODO --control-plane-endpoint
		commands := []string{
			fmt.Sprintf(`kubeadm init --pod-network-cidr=%s --apiserver-advertise-address=%s`, flannelCidr.String(), node.PrivateIPAddress),
			`mkdir -p $HOME/.kube`,
			`sudo cp -i /etc/kubernetes/admin.conf $HOME/.kube/config`,
			`sudo chown $(id -u):$(id -g) $HOME/.kube/config`,
			//fmt.Sprintf(`echo "%s" >> /etc/kube-flannel/net-conf.json`, flannelNetConf),
			//`mkdir /run/flannel/`,
			//fmt.Sprintf(`echo "%s" >> /run/flannel/subnet.env`, flannelRunConf),
			`kubectl apply -f https://raw.githubusercontent.com/coreos/flannel/master/Documentation/kube-flannel.yml`,
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

func getAllHosts(ip net.IP, ipNet net.IPNet) ([]string, int, error) {

	var ips []string
	for ip := ip.Mask(ipNet.Mask); ipNet.Contains(ip); inc(ip) {
		ips = append(ips, ip.String())
	}

	// remove network address and broadcast address
	lenIPs := len(ips)
	switch {
	case lenIPs < 2:
		return ips, lenIPs, nil

	default:
		return ips[1 : len(ips)-1], lenIPs - 2, nil
	}
}

func inc(ip net.IP) {
	for j := len(ip) - 1; j >= 0; j-- {
		ip[j]++
		if ip[j] > 0 {
			break
		}
	}
}
