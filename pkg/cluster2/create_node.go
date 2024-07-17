package cluster2

import (
	"context"
	"errors"
	"fmt"
	"github.com/xetys/hetzner-kube/pkg/clustermanager"
	"github.com/xetys/hetzner-kube/types"
	"path/filepath"
	"time"
)

func (c *Cluster) createNode(ctx context.Context, nodeTemplate types.NodeConfig) (clustermanager.Node, error) {
	nodeTemplate.Labels[types.ClusterNameLabel] = c.Config.Metadata.Name
	nodeTemplate.Labels[types.ClusterRoleLabel] = string(types.MasterNodeRole)

	sshClient := clustermanager.NewSSHCommunicator([]clustermanager.SSHKey{
		{
			Name:           c.Config.Metadata.Name,
			PrivateKeyPath: filepath.Join("./.ssh/", c.Config.Metadata.Name),
			PublicKeyPath:  filepath.Join("./.ssh/", fmt.Sprintf("%s.pub", c.Config.Metadata.Name)),
		},
	}, false)

	err := sshClient.(*clustermanager.SSHCommunicator).CapturePassphrase(c.Config.Metadata.Name)
	if err != nil {
		return clustermanager.Node{}, err
	}

	node, err := c.provider.CreateNode2(ctx, nodeTemplate)
	if err != nil {
		return clustermanager.Node{}, err
	}

	fmt.Printf("wait for cloud-init completion with deadline 300s")
	const tick = time.Second * 10
	ctxDeadline, cancelFunc := context.WithTimeout(ctx, time.Minute*5)
	defer cancelFunc()

	var counter int
	var t time.Time
	var isOk bool
	for {
		if t, isOk = ctxDeadline.Deadline(); !isOk {
			return clustermanager.Node{}, errors.New("cloud init deadline excited")
		}

		_, err := sshClient.RunCmd(node, "[ -f /tmp/hetzner-kube.unlock ]")
		if err == nil {
			fmt.Printf("cloud init complete with time: %d seconds\n", t.Second())
			break
		}
		//println(err.Error())
		println()
		counter++
		print(fmt.Sprintf("...%ds", int(tick.Seconds())*counter))
		time.Sleep(tick)
	}

	_, err = sshClient.RunCmd(node, "rm /tmp/hetzner-kube.unlock")
	if err != nil {
		fmt.Printf("WARN! can't delete cloud-init lock file")
	}

	return clustermanager.Node{}, nil
}
