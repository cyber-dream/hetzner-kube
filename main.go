package main

import (
	"fmt"
	"github.com/leaanthony/clir"
	"github.com/xetys/hetzner-kube/cluster"
	"github.com/xetys/hetzner-kube/pkg/clustermanager"
	"github.com/xetys/hetzner-kube/types"
)

type GetGenericFlag struct {
	Name string `pos:"1" description:"name of an object"`
}

type CreateClusterFlags struct {
	Name string `pos:"1" description:"Override random name of new cluster"`
	//Age  int    `pos:"2" description:"The age of the person" default:"20"`
}

func main() {
	//cmd.Execute()

	// Create new cli
	cli := clir.NewCli("Hetzner Kube", "A hetzner k8s manager", "v0.0.1")

	cli.Action(func() error {
		return nil
	})
	//fmt.Printf("Hello %s!\n", name)
	// Define action for the command
	get := cli.NewSubCommand("get", "get an object")
	getCluster := get.NewSubCommand("cluster", "todo")
	getCLusterFlags := GetGenericFlag{}
	getCluster.AddFlags(&getCLusterFlags)
	getCluster.Action(func() error {
		cluster.GetCluster(getCLusterFlags.Name)
		return nil
	})

	create := cli.NewSubCommand("create", "create an object")
	createCluster := create.NewSubCommand("cluster", "create a cluster")
	clusterCreationFlags := CreateClusterFlags{}
	createCluster.AddFlags(&clusterCreationFlags)
	createCluster.Action(func() error {
		err := cluster.RunClusterCreate(types.ClusterConfig{
			MasterNode:    types.MasterNodeConfig{},
			ClusterName:   clusterCreationFlags.Name,
			SSHKey:        clustermanager.SSHKey{},
			HetznerApiKey: "",
		})

		return err
	})
	if err := cli.Run(); err != nil {
		fmt.Printf("Error encountered: %v\n", err)
	}
}
