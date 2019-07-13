// Copyright © 2019 Alvaro Saurin
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package provisioner

import (
	"fmt"

	"github.com/hashicorp/terraform/helper/schema"

	"github.com/inercia/terraform-provider-kubeadm/internal/ssh"
	"github.com/inercia/terraform-provider-kubeadm/pkg/common"
)

// doKubeadmInit runs the `kubeadm init`
func doKubeadmInit(d *schema.ResourceData) ssh.Action {
	extraArgs := []string{"--skip-token-print"}

	// get the join configuration
	initConfig, _, err := common.InitConfigFromResourceData(d)
	if err != nil {
		return ssh.ActionError(fmt.Sprintf("could not get a valid 'config' for join'ing: %s", err))
	}

	// ... update the nodename
	initConfig.NodeRegistration.Name = getNodenameFromResourceData(d)

	// ... and update the `config.join` section
	if err := common.InitConfigToResourceData(d, initConfig); err != nil {
		return ssh.ActionError(err.Error())
	}

	actions := ssh.ActionList{
		ssh.DoMessageInfo("Initializing the cluster with 'kubadm init'..."),
		doDeleteLocalKubeconfig(d),
		doUploadCerts(d),
		// check if there is a (valid) admin.conf there
		// in that case, skip the "kubeadm init"
		ssh.DoIfElse(
			checkAdminConfAlive(d),
			ssh.DoMessageWarn("admin.conf already exists: skipping `kubeadm init`"),
			doKubeadm(d, "init", extraArgs...),
		),
		doDownloadKubeconfig(d),
		doLoadCNI(d),
		doLoadDashboard(d),
		doLoadHelm(d),
		doLoadManifests(d),
	}
	return actions
}
