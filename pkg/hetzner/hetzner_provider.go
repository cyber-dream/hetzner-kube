package hetzner

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"github.com/go-kit/kit/log/term"
	"github.com/gosuri/uiprogress"
	"github.com/hetznercloud/hcloud-go/v2/hcloud"
	"github.com/xetys/hetzner-kube/pkg/clustermanager"
	"github.com/xetys/hetzner-kube/types"
	"golang.org/x/crypto/ssh"
	"log"
	"math/rand"
	"net"
	"net/http"
	"os"
	"strings"
	"time"
)

// Provider contains providers information
type Provider struct {
	client        *hcloud.Client
	context       context.Context
	nodes         []clustermanager.Node
	clusterName   string
	cloudInitFile string
	wait          bool
	token         string //TODO Delete
	nodeCidr      string
	SSHKeyName    string
}

// NewHetznerProvider returns an instance of hetzner.Provider
func NewHetznerProvider(context context.Context, client *hcloud.Client, cluster clustermanager.Cluster, token string) *Provider {
	return &Provider{
		client:        client,
		context:       context,
		token:         token,
		nodeCidr:      cluster.NodeCIDR,
		clusterName:   cluster.Name,
		cloudInitFile: cluster.CloudInitFile,
		nodes:         cluster.Nodes,
	}
}

const appName = "hetzner-kube"

func NewHetznerProvider2(context context.Context, htoken string, clusterName string) *Provider {
	return &Provider{
		client:        hcloud.NewClient(hcloud.WithToken(htoken), hcloud.WithApplication(appName, "0.0.1")),
		context:       context,
		token:         htoken,
		nodeCidr:      "10.0.1.0/24",
		clusterName:   clusterName,
		cloudInitFile: "TODO DELETE",
		//nodes:         cluster.Nodes,
		SSHKeyName: clusterName, //TODO Delete this field, uses in SSHClient
	}
}

const ()

// CreateNodes creates hetzner nodes
//
// Deprecated:
func (provider *Provider) CreateNode(ctx context.Context, suffix types.NodeRole, template clustermanager.NodeTemplate, count int, offset int) ([]clustermanager.Node, error) {
	sshKey, _, err := provider.client.SSHKey.Get(provider.context, provider.SSHKeyName)

	if err != nil {
		return nil, err
	}

	if sshKey == nil {
		return nil, fmt.Errorf("we got some problem with the SSH-Key '%s', chances are you are in the wrong context", provider.SSHKeyName)
	}

	serverNameTemplate := fmt.Sprintf("%s-%s-@idx", provider.clusterName, suffix)
	serverOptsTemplate := hcloud.ServerCreateOpts{
		Name: serverNameTemplate,
		ServerType: &hcloud.ServerType{
			Name: template.ServerType,
		},
		Image: &hcloud.Image{
			Name: template.Image,
		},
		//UserData: template.CloudInit,FIXME
	}

	serverOptsTemplate.Labels = make(map[string]string)
	serverOptsTemplate.Labels[types.ClusterNameLabel] = provider.clusterName
	if suffix == "master" {
		serverOptsTemplate.Labels[types.ClusterRoleLabel] = string(types.MasterNodeRole)
	} else {
		serverOptsTemplate.Labels[types.ClusterRoleLabel] = string(types.WorkerNodeRole)
	} //TODO switch with etcd

	serverOptsTemplate.SSHKeys = append(serverOptsTemplate.SSHKeys, sshKey)

	datacentersCount := len(template.DataCenters)

	//shuffle datacenters to make it more random
	rand.Seed(time.Now().UnixNano())
	rand.Shuffle(datacentersCount, func(i, j int) {
		template.DataCenters[i], template.DataCenters[j] = template.DataCenters[j], template.DataCenters[i]
	})

	var nodes []clustermanager.Node
	for i := 1; i <= count; i++ {
		serverOpts := serverOptsTemplate
		nodeNumber := i + offset
		serverOpts.Name = strings.Replace(serverNameTemplate, "@idx", fmt.Sprintf("%.02d", nodeNumber), 1)
		serverOpts.Datacenter = &hcloud.Datacenter{
			Name: template.DataCenters[i%datacentersCount],
		}

		// create
		server, err := provider.runCreateServer(ctx, &serverOpts)

		if err != nil {
			return nil, err
		}

		ipAddress := server.Server.PublicNet.IPv4.IP.String()
		log.Printf("Created node '%s' with IP %s", server.Server.Name, ipAddress)

		// render private IP address
		privateIPLastBlock := nodeNumber
		//FIXME
		//if !template.IsEtcd {
		//	privateIPLastBlock += 10
		//	if !template.IsMaster {
		//		privateIPLastBlock += 10
		//	}
		//}
		cidrPrefix, err := clustermanager.PrivateIPPrefix(provider.nodeCidr)
		if err != nil {
			return nil, err
		}

		privateIPAddress := fmt.Sprintf("%s.%d", cidrPrefix, privateIPLastBlock)

		node := clustermanager.Node{
			Name:             serverOpts.Name,
			Type:             serverOpts.ServerType.Name,
			IsMaster:         suffix == "master", //FIXME
			IsEtcd:           false,              //FIXME
			IPAddress:        ipAddress,
			PrivateIPAddress: privateIPAddress,
			SSHKeyName:       provider.SSHKeyName,
		}
		nodes = append(nodes, node)
		provider.nodes = append(provider.nodes, node)
	}

	return nodes, nil
}

func (provider *Provider) CreateNode2(ctx context.Context, nodeTemplate types.NodeConfig) (clustermanager.Node, error) {
	//all, err := provider.client.Image.All(ctx)
	//if err != nil {
	//	return clustermanager.Node{}, err
	//}
	//_ = all

	role, isFound := nodeTemplate.Labels[types.ClusterRoleLabel]
	if !isFound {
		return clustermanager.Node{}, errors.New("role label is empty")
	}

	sshKey, _, err := provider.client.SSHKey.Get(provider.context, provider.SSHKeyName)
	if err != nil {
		return clustermanager.Node{}, err
	}

	cloudConfRender, err := nodeTemplate.CloudInit.RenderYAML()
	if err != nil {
		return clustermanager.Node{}, err
	}

	nodes, err := provider.GetAllNodes2(ctx, types.MasterNodeRole) //FIXME hardcoded
	nodeNumber := len(nodes)
	serverOptsTemplate := hcloud.ServerCreateOpts{
		Name: fmt.Sprintf("%s-%s-%d", provider.clusterName, role, nodeNumber),
		ServerType: &hcloud.ServerType{
			Name: nodeTemplate.Type,
		},
		Image: &hcloud.Image{
			Name: nodeTemplate.Image,
		},
		Location: &hcloud.Location{
			Name: "nbg1",
		},
		Networks: nodeTemplate.Networks,
		UserData: string(cloudConfRender),
		Labels:   nodeTemplate.Labels,
		SSHKeys:  []*hcloud.SSHKey{sshKey},
	}

	//shuffle datacenters to make it more random
	rand.Seed(time.Now().UnixNano())
	rand.Shuffle(len(nodeTemplate.DataCenters), func(i, j int) {
		nodeTemplate.DataCenters[i], nodeTemplate.DataCenters[j] = nodeTemplate.DataCenters[j], nodeTemplate.DataCenters[i]
	})

	res, err := provider.runCreateServer(ctx, &serverOptsTemplate)
	if err != nil {
		return clustermanager.Node{}, err
	}
	if res == nil {
		return clustermanager.Node{}, errors.New("result from hetzner api is nil")
	}

	ipAddress := res.Server.PublicNet.IPv4.IP.String()
	log.Printf("Created node '%s' with IP %s", res.Server.Name, ipAddress)

	// render private IP address
	privateIPLastBlock := nodeNumber
	//FIXME
	//if !template.IsEtcd {
	//	privateIPLastBlock += 10
	//	if !template.IsMaster {
	//		privateIPLastBlock += 10
	//	}
	//}
	cidrPrefix, err := clustermanager.PrivateIPPrefix(provider.nodeCidr)
	if err != nil {
		return clustermanager.Node{}, err
	}

	privateIPAddress := fmt.Sprintf("%s.%d", cidrPrefix, privateIPLastBlock)

	node := clustermanager.Node{
		Name:             res.Server.Name,
		Type:             res.Server.ServerType.Name,
		IsMaster:         serverOptsTemplate.Labels[types.ClusterRoleLabel] == "master", //FIXME
		IsEtcd:           false,                                                         //FIXME
		IPAddress:        ipAddress,
		PrivateIPAddress: privateIPAddress,
		SSHKeyName:       provider.SSHKeyName,
	}

	return node, nil
}

// CreateEtcdNodes creates nodes with type 'etcd'
func (provider *Provider) CreateEtcdNodes(sshKeyName string, masterServerType string, datacenters []string, count int) ([]clustermanager.Node, error) {
	//template := clustermanager.Node{SSHKeyName: sshKeyName, IsEtcd: true, Type: masterServerType}
	//return providers.CreateNode("etcd", template, datacenters, count, 0, sshKeyName)
	//FIXME
	return nil, errors.New("fixme not implemented")
}

var tempDatacenters = []string{"nbg1-dc3"} //FIXME
// CreateMasterNodes creates nodes with type 'master'
// Deprecated:
func (provider *Provider) CreateMasterNodes(params clustermanager.Cluster, isEtcd bool) ([]clustermanager.Node, error) {
	//return providers.CreateNode("master", params.MasterTemplate, tempDatacenters, params.MastersCount, 0)
	return nil, nil
}

// CreateWorkerNodes create new worker node on providers
// Deprecated:
func (provider *Provider) CreateWorkerNodes(params clustermanager.Cluster, count int, offset int) ([]clustermanager.Node, error) {
	//return providers.CreateNode("worker", params.WorkerTemplate, tempDatacenters, count, offset, params.SSHKeyName)
	return nil, nil
}

// GetAllNodes retrieves all nodes
func (provider *Provider) GetAllNodes() []clustermanager.Node {
	return provider.nodes
}

func (provider *Provider) GetAllNodes2(ctx context.Context, filter types.NodeRole) ([]clustermanager.Node, error) {
	allServers, err := provider.client.Server.All(ctx)
	if err != nil {
		return nil, err
	}

	var nodes []clustermanager.Node
	for _, s := range allServers {
		role := s.Labels[types.ClusterRoleLabel]
		if role != string(filter) && role != string(types.AnyNodeRole) {
			continue
		}

		nodes = append(nodes, clustermanager.Node{Name: s.Name})
	}

	return nodes, nil
}

// SetNodes set list of cluster nodes for this providers
func (provider *Provider) SetNodes(nodes []clustermanager.Node) {
	provider.nodes = nodes
}

// GetMasterNodes returns master nodes only
func (provider *Provider) GetMasterNodes() []clustermanager.Node {
	return provider.filterNodes(func(node clustermanager.Node) bool {
		return node.IsMaster
	})
}

// GetEtcdNodes returns etcd nodes only
func (provider *Provider) GetEtcdNodes() []clustermanager.Node {
	return provider.filterNodes(func(node clustermanager.Node) bool {
		return node.IsEtcd
	})
}

// GetWorkerNodes returns worker nodes only
func (provider *Provider) GetWorkerNodes() []clustermanager.Node {
	return provider.filterNodes(func(node clustermanager.Node) bool {
		return !node.IsMaster && !node.IsEtcd
	})
}

// GetMasterNode returns the first master node or fail, if no master nodes are found
func (provider *Provider) GetMasterNode() (*clustermanager.Node, error) {
	nodes := provider.GetMasterNodes()
	if len(nodes) == 0 {
		return nil, errors.New("no master node found")
	}

	return &nodes[0], nil
}

// GetCluster returns a template for Cluster
func (provider *Provider) GetCluster() clustermanager.Cluster {
	return clustermanager.Cluster{
		Name:          provider.clusterName,
		Nodes:         provider.nodes,
		CloudInitFile: provider.cloudInitFile,
		NodeCIDR:      provider.nodeCidr,
	}
}

// GetAdditionalMasterInstallCommands return the list of node command to execute on the cluster
func (provider *Provider) GetAdditionalMasterInstallCommands() []clustermanager.NodeCommand {

	return []clustermanager.NodeCommand{}
}

// GetNodeCidr returns the CIDR to use for nodes in cluster
func (provider *Provider) GetNodeCidr() string {
	return provider.nodeCidr
}

// MustWait returns true, if we have to wait after creation for some time
func (provider *Provider) MustWait() bool {
	return provider.wait
}

// Token returns the hcloud token
func (provider *Provider) Token() string {
	return provider.token
}

type nodeFilter func(clustermanager.Node) bool

func (provider *Provider) filterNodes(filter nodeFilter) []clustermanager.Node {
	nodes := []clustermanager.Node{}
	for _, node := range provider.nodes {
		if filter(node) {
			nodes = append(nodes, node)
		}
	}

	return nodes
}

func (provider *Provider) runCreateServer(ctx context.Context, opts *hcloud.ServerCreateOpts) (*hcloud.ServerCreateResult, error) {
	//log.Printf("creating server '%s'...", opts.Name)
	server, _, err := provider.client.Server.GetByName(ctx, opts.Name)
	if err != nil {
		return nil, err
	}
	if server != nil {
		return nil, errors.New("servers name collision")
	}

	serverCreateResult, _, err := provider.client.Server.Create(ctx, *opts)
	if err != nil {
		return nil, err
	}

	actions := append(serverCreateResult.NextActions, serverCreateResult.Action)
	err = provider.client.Action.WaitFor(ctx, actions...)
	if err != nil {
		//_ = provider.c.deleteServer(server) TODO
		return nil, fmt.Errorf("failed to start server %s error: %v", server.Name, err)
	}

	log.Printf("loading server '%s'...", opts.Name)
	return &hcloud.ServerCreateResult{Server: serverCreateResult.Server}, nil
}

func (provider *Provider) actionProgress(action *hcloud.Action) error {
	progressCh, errCh := provider.client.Action.WatchProgress(provider.context, action)

	if term.IsTerminal(os.Stdout) {
		progress := uiprogress.New()

		progress.Start()
		bar := progress.AddBar(100).AppendCompleted().PrependElapsed()
		bar.Width = 40
		bar.Empty = ' '

		for {
			select {
			case err := <-errCh:
				if err == nil {
					bar.Set(100)
				}
				progress.Stop()
				return err
			case p := <-progressCh:
				bar.Set(p)
			}
		}
	} else {
		return <-errCh
	}
}

func (provider *Provider) RawCli() *hcloud.Client {
	return provider.client
}

func (provider *Provider) AddSSHKey(ctx context.Context, name string, publicKey []byte) error {
	opts := hcloud.SSHKeyCreateOpts{
		Name:      name,
		PublicKey: string(publicKey),
	}

	sshKey, res, err := provider.client.SSHKey.Create(ctx, opts)

	if res.StatusCode == http.StatusConflict {
		pkey, _, _, _, err := ssh.ParseAuthorizedKey(publicKey)
		if err != nil {
			log.Fatalln(err)
		}

		sshKeyHetzner, _, err := provider.client.SSHKey.Get(ctx, name)
		if err != nil {
			return err
		}

		hetznerPkey, _, _, _, err := ssh.ParseAuthorizedKey([]byte(sshKeyHetzner.PublicKey))
		if err != nil {
			return err
		}

		if bytes.Equal(pkey.Marshal(), hetznerPkey.Marshal()) {
			fmt.Printf("SSH key does already exist on hetzner as '%s'\n", sshKeyHetzner.Name)
		}
	} else if err != nil {
		return err
	} else {
		fmt.Printf("SSH key %s(%d) created\n", sshKey.Name, sshKey.ID)
	}

	return nil
}

func (provider *Provider) CreateNetwork(ctx context.Context, clusterName string, ipRange *net.IPNet) (*hcloud.Network, error) {
	network, _, err := provider.client.Network.Create(ctx, hcloud.NetworkCreateOpts{
		Name:    fmt.Sprintf("%s-generated", clusterName),
		IPRange: ipRange,
		Subnets: []hcloud.NetworkSubnet{
			{
				Type:        "cloud",
				IPRange:     ipRange,
				NetworkZone: hcloud.NetworkZoneEUCentral, //FIXME Hardcoded
				Gateway:     nil,
				VSwitchID:   0,
			},
		},
		Labels: map[string]string{
			types.ClusterNameLabel: clusterName,
		},
		ExposeRoutesToVSwitch: false,
	})

	return network, err
}

func (provider *Provider) CreateFirewall(ctx context.Context, name string, ports []string) (*hcloud.Firewall, error) {
	_, tempIpNet, err := net.ParseCIDR("195.201.229.4/32") //FIXME hardcoded ip
	if err != nil {
		return nil, err
	}

	opts := hcloud.FirewallCreateOpts{
		Name: fmt.Sprintf("%s-generated", name),
		Labels: map[string]string{
			types.ClusterNameLabel: name,
		},

		ApplyTo: []hcloud.FirewallResource{
			{
				Type:   hcloud.FirewallResourceTypeLabelSelector,
				Server: nil,
				LabelSelector: &hcloud.FirewallResourceLabelSelector{
					Selector: "cluster-role",
				},
			},
		},
	}

	for _, port := range ports {
		p := port
		opts.Rules = append(opts.Rules, hcloud.FirewallRule{
			Direction:      hcloud.FirewallRuleDirectionIn,
			SourceIPs:      []net.IPNet{*tempIpNet}, //[]net.IPNet{*all1, *all2},
			DestinationIPs: nil,
			Protocol:       hcloud.FirewallRuleProtocolTCP,
			Port:           &p,
			Description:    nil,
		})
	}

	create, _, err := provider.client.Firewall.Create(ctx, opts)

	return create.Firewall, err
}
