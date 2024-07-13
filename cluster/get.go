package cluster

import (
	"context"
	"github.com/hetznercloud/hcloud-go/v2/hcloud"
	"github.com/jedib0t/go-pretty/v6/table"
	"github.com/xetys/hetzner-kube/appconf"
	"github.com/xetys/hetzner-kube/types"
	"os"
)

func GetCluster(clusterName string) {
	ctx := context.Background()
	conf, _ := appconf.GetConfig(clusterName)

	cli := hcloud.NewClient(hcloud.WithToken(conf.HetznerApiKey), hcloud.WithApplication("todo", "0.0.1"))

	allServers, err := cli.Server.All(ctx)
	if err != nil {
		return
	}

	var masterServers []table.Row
	var workerServers []table.Row
	var etcdServers []table.Row

	for _, server := range allServers {
		// TODO use const names
		clusterNameLabel, isOK := server.Labels[types.ClusterNameLabel]
		if !isOK {
			continue
		}

		// TODO use const names
		roleLabel, isOK := server.Labels[types.ClusterRoleLabel]
		if !isOK {
			continue
		}

		if clusterNameLabel != clusterName {
			continue
		}

		switch roleLabel {
		case string(types.MasterNodeRole):
			masterServers = append(masterServers, table.Row{server.ID, server.Name, server.PublicNet.IPv4.IP.String()})
		case string(types.WorkerNodeRole):
			workerServers = append(workerServers, table.Row{server.ID, server.Name, server.PublicNet.IPv4.IP.String()})
		case string(types.EtcdNodeRole):
			etcdServers = append(etcdServers, table.Row{server.ID, server.Name, server.PublicNet.IPv4.IP.String()})
		}
	}

	if len(masterServers) < 1 {
		println("cluster does not exist")
	}

	t := table.NewWriter()
	t.SetStyle(types.TableStyle)
	t.SetOutputMirror(os.Stdout)
	t.AppendHeader(table.Row{"ID", "Name", "Public IP"})
	t.AppendRows(masterServers)
	t.AppendSeparator()
	t.AppendRows(workerServers)
	t.AppendSeparator()
	t.AppendRows(etcdServers)
	t.Render()
}
