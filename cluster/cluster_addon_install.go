package cluster

import (
	"github.com/spf13/cobra"
)

// clusterAddonInstallCmd represents the clusterAddonInstall command
var clusterAddonInstallCmd = &cobra.Command{
	//Use:     "install",
	//Short:   "installs an addon to a cluster",
	//PreRunE: validateAddonSubCommand,
	//Run: func(cmd *cobra.Command, args []string) {
	//	name, _ := cmd.Flags().GetString("name")
	//	addonName := args[0]
	//
	//	_, cluster := AppConf.Config.FindClusterByName(name)
	//
	//	log.Printf("installing addon %s", addonName)
	//	providers := hetzner.NewHetznerProvider(AppConf.Context, AppConf.Client, *cluster, AppConf.CurrentContext.Token)
	//	addonService := addons.NewClusterAddonService(providers, AppConf.SSHClient)
	//	masterNode, err := providers.GetMasterNode()
	//	FatalOnError(err)
	//
	//	err = AppConf.SSHClient.(*clustermanager.SSHCommunicator).CapturePassphrase(masterNode.SSHKeyName)
	//	FatalOnError(err)
	//
	//	addon := addonService.GetAddon(addonName)
	//	addon.Install()
	//
	//	log.Printf("addon %s successfully installed", addonName)
	//},
}

func init() {
	clusterAddonCmd.AddCommand(clusterAddonInstallCmd)

	clusterAddonInstallCmd.Flags().StringP("name", "n", "", "Name of the cluster")
}
