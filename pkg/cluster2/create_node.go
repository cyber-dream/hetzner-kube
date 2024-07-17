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
	sshClient := clustermanager.NewSSHCommunicator([]clustermanager.SSHKey{
		{
			Name:           c.Config.Metadata.Name,
			PrivateKeyPath: filepath.Join("./.ssh/", c.Config.Metadata.Name), //FIXME Hardcoded
			PublicKeyPath:  filepath.Join("./.ssh/", fmt.Sprintf("%s.pub", c.Config.Metadata.Name)),
		},
	}, false)

	err := sshClient.(*clustermanager.SSHCommunicator).CapturePassphrase(c.Config.Metadata.Name)
	if err != nil {
		return clustermanager.Node{}, err
	}

	nodeTemplate.CloudInit.AddRunCmd("touch /tmp/hetzner-kube.unlock")

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

		_, err := sshClient.RunCmd(node, "[ -f /tmp/hetzner-kube.unlock ]")
		if err == nil {
			fmt.Printf("\ncloud init complete with time: %d seconds\n", t.Second())
			break
		}

		counter++
		print(fmt.Sprintf("...%ds", int(tick.Seconds())*counter))
		time.Sleep(tick)
	}

	commands := []string{
		`kubeadm init`,
		`mkdir -p $HOME/.kube`,
		`sudo cp -i /etc/kubernetes/admin.conf $HOME/.kube/config`,
		`sudo chown $(id -u):$(id -g) $HOME/.kube/config`,
		`kubectl apply -f https://raw.githubusercontent.com/coreos/flannel/master/Documentation/kube-flannel.yml`,
	}

	for _, command := range commands {
		println("executing: ", command)
		data, err := sshClient.RunCmd(node, command)
		if err != nil {
			fmt.Printf(err.Error())
			break
		}

		println(data)
	}
	return clustermanager.Node{}, nil
}
