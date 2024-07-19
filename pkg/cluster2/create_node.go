package cluster2

import (
	"context"
	"errors"
	"fmt"
	"github.com/xetys/hetzner-kube/pkg/clustermanager"
	"github.com/xetys/hetzner-kube/pkg/hetzner"
	"github.com/xetys/hetzner-kube/types"
	"path/filepath"
	"time"
)

func (c *Cluster) createNode(ctx context.Context, nodeTemplate types.ProviderNodeTemplate) (clustermanager.Node, error) {
	sshClient := clustermanager.NewSSHCommunicator([]clustermanager.SSHKey{
		{
			Name:           c.Config.Metadata.Name,
			PrivateKeyPath: filepath.Join("./.ssh/", c.Config.Metadata.Name), //FIXME Hardcoded
			PublicKeyPath:  filepath.Join("./.ssh/", fmt.Sprintf("%s.pub", c.Config.Metadata.Name)),
		},
	}, false)

	nodeTemplate.SSHKeyName = c.Config.Metadata.Name //FIXME
	err := sshClient.(*clustermanager.SSHCommunicator).CapturePassphrase(c.Config.Metadata.Name)
	if err != nil {
		return clustermanager.Node{}, err
	}

	nodes, err := c.provider.GetAllNodes2(ctx, hetzner.Selector{
		Labels: map[string]string{
			types.ClusterNameLabel: c.Config.Metadata.Name,
			types.ClusterRoleLabel: nodeTemplate.Labels[types.ClusterRoleLabel],
		},
	})
	if err != nil {
		return clustermanager.Node{}, err
	}

	nodeTemplate.CloudInit.AddRunCmd("touch /tmp/hetzner-kube.unlock")
	nodeTemplate.Name = fmt.Sprintf("%s-%s-%d", c.Config.Metadata.Name, nodeTemplate.Role, len(nodes))
	node, err := c.provider.CreateNode2(ctx, nodeTemplate)
	if err != nil {
		return clustermanager.Node{}, err
	}

	fmt.Printf("wait for cloud-init completion with deadline 300s\n")
	const tick = time.Second * 5
	ctxDeadline, cancelFunc := context.WithTimeout(ctx, time.Minute*5)
	defer cancelFunc()

	//FIXME Deadline not works
	var counter int
	var t time.Time
	var isOk bool
	for {
		if t, isOk = ctxDeadline.Deadline(); !isOk {
			return clustermanager.Node{}, errors.New("cloud init deadline excited")
		}

		data, err := sshClient.RunCmd(node, "[ -f /tmp/hetzner-kube.unlock ]")
		if err == nil {
			fmt.Printf("\ncloud init complete with time: %d seconds\n", t.Second())
			break
		}

		println(data)
		counter++
		print(fmt.Sprintf("...%ds", int(tick.Seconds())*counter))
		time.Sleep(tick)
	}

	for _, command := range nodeTemplate.Commands {
		println("executing: ", command)
		data, err := sshClient.RunCmd(node, command)
		if err != nil {
			fmt.Printf(err.Error())
			break
		}

		println(data)
	}

	return node, nil
}
