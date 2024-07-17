package main

import (
	"fmt"
	"github.com/leaanthony/clir"
	"github.com/xetys/hetzner-kube/cluster"
	"github.com/xetys/hetzner-kube/types"
	"gopkg.in/yaml.v3"
	"os"
	"strings"
)

type GetGenericFlag struct {
	Name string `pos:"1" description:"name of an object"`
}

type CreateClusterFlags struct {
	Name string `pos:"1" description:"Override random name of new cluster"`
	F    string `pos:"2" description:"Path to a config file"`
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
		var clusterConf types.ClusterConfig
		stat, err := os.Stat(clusterCreationFlags.F)
		if err == nil {
			if !stat.IsDir() {
				file, err := os.ReadFile(clusterCreationFlags.F)
				if err != nil {
					fmt.Printf("error in reading file %s: %s\n", clusterCreationFlags.F, err)
					return err
				}

				var rawConf map[string]interface{}
				err = yaml.Unmarshal(file, &rawConf)
				val, isOk := rawConf["kind"]
				if !isOk {
					fmt.Printf("error in parsing file %s: %s\n", clusterCreationFlags.F, "config kind not defined")
				}
				strVal, isOk := val.(string)
				if !isOk {
					fmt.Printf("error in parsing file %s: %s\n", clusterCreationFlags.F, "config kind is not a string")
				}
				if strings.ToLower(strVal) != "cluster" {
					fmt.Printf("error in parsing file %s: %s\n", clusterCreationFlags.F, "config kind is not a cluster")
				}

				err = yaml.Unmarshal(file, &clusterConf)
				if err != nil {
					fmt.Printf("error in parsing file %s: %s\n", clusterCreationFlags.F, err)
					return err
				}
			}
		}

		err = cluster.RunClusterCreate(clusterConf)

		return err

	})
	if err := cli.Run(); err != nil {
		fmt.Printf("Error encountered: %v\n", err)
	}
}
