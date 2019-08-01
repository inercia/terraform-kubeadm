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
	"strings"

	"github.com/hashicorp/terraform/helper/schema"

	"github.com/inercia/terraform-provider-kubeadm/internal/ssh"
	"github.com/inercia/terraform-provider-kubeadm/pkg/common"
)

// doLoadCNI loads the CNI driver
func doLoadCNI(d *schema.ResourceData) ssh.Action {
	manifest := ssh.Manifest{}
	var message ssh.Action

	if cniPluginManifestOpt, ok := d.GetOk("config.cni_plugin_manifest"); ok {
		cniPluginManifest := strings.TrimSpace(cniPluginManifestOpt.(string))
		if len(cniPluginManifest) > 0 {
			manifest = ssh.NewManifest(cniPluginManifest)
			if manifest.Inline != "" {
				return ssh.ActionError(fmt.Sprintf("%q not recognized as URL or local filename", cniPluginManifest))
			}
			message = ssh.DoMessageInfo(fmt.Sprintf("Loading CNI plugin from %q", cniPluginManifest))
		}
	} else {
		if cniPluginOpt, ok := d.GetOk("config.cni_plugin"); ok {
			cniPlugin := strings.TrimSpace(strings.ToLower(cniPluginOpt.(string)))
			if len(cniPlugin) > 0 {
				ssh.Debug("verifying CNI plugin: %s", cniPlugin)
				if m, ok := common.CNIPluginsManifestsTemplates[cniPlugin]; ok {
					ssh.Debug("CNI plugin: %s", cniPlugin)
					manifest = m
				} else {
					panic("unknown CNI driver: should have been caught at the validation stage")
				}
				message = ssh.DoMessageInfo(fmt.Sprintf("Loading CNI plugin %q", cniPlugin))
			}
		}
	}

	if manifest.IsEmpty() {
		return ssh.DoMessageWarn("no CNI driver is going to be loaded")
	}

	err := manifest.ReplaceConfig(common.GetProvisionerConfig(d))
	if err != nil {
		return ssh.ActionError(fmt.Sprintf("could not replace variables in manifest: %s", err))
	}

	return ssh.ActionList{
		message,
		doRemoteKubectlApply(d, []ssh.Manifest{manifest}),
	}
}
