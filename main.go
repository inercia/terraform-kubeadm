package main

import (
	"github.com/hashicorp/terraform/plugin"

	"github.com/inercia/terraform-kubeadm/kubeadm"
)

func main() {
	plugin.Serve(&plugin.ServeOpts{
		ProviderFunc:    kubeadm.Provider,
		ProvisionerFunc: kubeadm.Provisioner,
	})
}