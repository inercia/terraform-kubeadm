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
	"strconv"

	"github.com/hashicorp/terraform/helper/schema"

	"github.com/inercia/terraform-provider-kubeadm/internal/ssh"
	"github.com/inercia/terraform-provider-kubeadm/pkg/common"
)

// doLoadDashboard loads the dashboard (if enabled)
func doLoadDashboard(d *schema.ResourceData) ssh.Action {
	opt, ok := d.GetOk("config.dashboard_enabled")
	if !ok {
		return ssh.DoMessageWarn("the Dashboard will not be loaded")
	}
	enabled, err := strconv.ParseBool(opt.(string))
	if err != nil {
		return ssh.ActionError("could not parse dashboard_enabled in provisioner")
	}
	if !enabled {
		return ssh.DoMessageWarn("The Dashboard will not be loaded")
	}
	if common.DefDashboardManifest == "" {
		return ssh.DoMessageWarn("No manifest for Dashboard: the Dashboard will not be loaded")
	}
	return ssh.ActionList{
		ssh.DoMessageInfo(fmt.Sprintf("Loading Dashboard from %q", common.DefDashboardManifest)),
		doRemoteKubectlApply(d, []ssh.Manifest{{URL: common.DefDashboardManifest}}),
	}
}

// doLoadExtraManifests loads some extra manifests
func doLoadExtraManifests(d *schema.ResourceData) ssh.Action {
	manifestsOpt, ok := d.GetOk("manifests")
	if !ok {
		return nil
	}
	manifests := []ssh.Manifest{}
	for _, v := range manifestsOpt.([]interface{}) {
		manifests = append(manifests, ssh.NewManifest(v.(string)))
	}
	if len(manifests) == 0 {
		return ssh.DoMessageWarn("Could not find valid manifests to load")
	}
	return ssh.ActionList{
		ssh.DoMessageInfo(fmt.Sprintf("Loading %d extra manifests", len(manifests))),
		doRemoteKubectlApply(d, manifests),
	}
}
