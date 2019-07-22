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

package common

import (
	"errors"
	"fmt"

	"github.com/hashicorp/terraform/helper/schema"
	kubeadmapi "k8s.io/kubernetes/cmd/kubeadm/app/apis/kubeadm"
	kubeadmscheme "k8s.io/kubernetes/cmd/kubeadm/app/apis/kubeadm/scheme"
	kubeadmapiv1beta1 "k8s.io/kubernetes/cmd/kubeadm/app/apis/kubeadm/v1beta1"
	kubeadmutil "k8s.io/kubernetes/cmd/kubeadm/app/util"
	"k8s.io/kubernetes/cmd/kubeadm/app/util/config"
)

const (
	resourcePathInitConfig = "config.init"
	resourcePathJoinConfig = "config.join"
)

var (
	// group version used to registering these objects
	apiVersion = kubeadmapiv1beta1.SchemeGroupVersion
)

var (
	errNoInitConfigFound = errors.New("no init configuration obtained")
	errNoJoinConfigFound = errors.New("no join configuration obtained")
)

//
// Init
//

// YAMLToInitConfig converts a YAML to InitConfiguration
func YAMLToInitConfig(configBytes []byte) (*kubeadmapi.InitConfiguration, error) {
	var initConfig *kubeadmapi.InitConfiguration
	var clusterConfig *kubeadmapi.ClusterConfiguration

	objects, err := kubeadmutil.SplitYAMLDocuments(configBytes)
	if err != nil {
		return nil, err
	}
	for k, v := range objects {
		if kubeadmutil.GroupVersionKindsHasInitConfiguration(k) {
			obj, err := kubeadmutil.UnmarshalFromYamlForCodecs(v, kubeadmapi.SchemeGroupVersion, kubeadmscheme.Codecs)
			if err != nil {
				return nil, err
			}

			cfg2, ok := obj.(*kubeadmapi.InitConfiguration)
			if !ok || cfg2 == nil {
				return nil, err
			}

			initConfig = cfg2
		} else if kubeadmutil.GroupVersionKindsHasClusterConfiguration(k) {
			obj, err := kubeadmutil.UnmarshalFromYamlForCodecs(v, kubeadmapi.SchemeGroupVersion, kubeadmscheme.Codecs)
			if err != nil {
				return nil, err
			}

			cfg2, ok := obj.(*kubeadmapi.ClusterConfiguration)
			if !ok || cfg2 == nil {
				return nil, err
			}

			clusterConfig = cfg2
		}
	}

	if initConfig != nil && clusterConfig != nil {
		initConfig.ClusterConfiguration = *clusterConfig
	}

	return initConfig, nil
}

// InitConfigToYAML converts a InitConfiguration to YAML
func InitConfigToYAML(initConfig *kubeadmapi.InitConfiguration) ([]byte, error) {
	kubeadmscheme.Scheme.Default(initConfig)
	return config.MarshalInitConfigurationToBytes(initConfig, apiVersion)
}

// InitConfigFromResourceData unmarshalls the initConfiguration passed from
// the kubeadm `data` resource
func InitConfigFromResourceData(d *schema.ResourceData) (*kubeadmapi.InitConfiguration, []byte, error) {

	cfg, ok := d.GetOk(resourcePathInitConfig)
	if !ok {
		return nil, nil, errNoInitConfigFound
	}

	// deserialize the configuration saved in the `config`
	configBytes, err := FromTerraformSafeString(cfg.(string))
	if err != nil {
		return nil, nil, err
	}

	// load the initConfiguration from the `config` field
	initConfig, err := YAMLToInitConfig(configBytes)
	if err != nil {
		return nil, nil, err
	}

	// ... update some things, like the seeder, the nodename, etc
	if nodenameOpt, ok := d.GetOk("nodename"); ok {
		initConfig.NodeRegistration.Name = nodenameOpt.(string)
	}

	configBytes, err = InitConfigToYAML(initConfig)
	if err != nil {
		return nil, nil, err
	}

	// ssh.Debug("init config:\n%s\n", configBytes)
	return initConfig, configBytes, nil
}

// InitConfigToResourceData updates the `config.init` in the ResourceData
// with the initConfig provided
func InitConfigToResourceData(d *schema.ResourceData, initConfig *kubeadmapi.InitConfiguration) error {
	initConfigBytes, err := InitConfigToYAML(initConfig)
	if err != nil {
		return fmt.Errorf("could not get a valid 'config' for init'ing: %s", err)
	}

	// update the config.init
	config := d.Get("config").(map[string]interface{})
	config["init"] = ToTerraformSafeString(initConfigBytes[:])
	if err := d.Set("config", config); err != nil {
		return fmt.Errorf("cannot update config.init")
	}
	return nil
}

//
// Join
//

// YAMLToJoinConfig converts a YAML to JoinConfiguration
func YAMLToJoinConfig(configBytes []byte) (*kubeadmapi.JoinConfiguration, error) {
	var joinConfig *kubeadmapi.JoinConfiguration

	objects, err := kubeadmutil.SplitYAMLDocuments(configBytes)
	if err != nil {
		return nil, err
	}
	for k, v := range objects {
		if kubeadmutil.GroupVersionKindsHasJoinConfiguration(k) {
			obj, err := kubeadmutil.UnmarshalFromYamlForCodecs(v, kubeadmapi.SchemeGroupVersion, kubeadmscheme.Codecs)
			if err != nil {
				return nil, err
			}

			cfg2, ok := obj.(*kubeadmapi.JoinConfiguration)
			if !ok || cfg2 == nil {
				return nil, err
			}

			joinConfig = cfg2
		}
	}

	return joinConfig, nil
}

// JoinConfigToYAML converts a JoinConfiguration to YAML
func JoinConfigToYAML(joinConfig *kubeadmapi.JoinConfiguration) ([]byte, error) {
	kubeadmscheme.Scheme.Default(joinConfig)
	nodebytes, err := kubeadmutil.MarshalToYamlForCodecs(joinConfig, apiVersion, kubeadmscheme.Codecs)
	if err != nil {
		return []byte{}, err
	}

	return nodebytes, nil
}

// JoinConfigFromResourceData unmarshalls the joinConfiguration passed from
// the kubeadm `data` resource
func JoinConfigFromResourceData(d *schema.ResourceData) (*kubeadmapi.JoinConfiguration, []byte, error) {

	var err error
	var joinConfig *kubeadmapi.JoinConfiguration

	seeder := d.Get("join").(string)
	cfg, ok := d.GetOk(resourcePathJoinConfig)
	if !ok {
		return nil, nil, errNoJoinConfigFound
	}

	// deserialize the configuration saved in the `config`
	configBytes, err := FromTerraformSafeString(cfg.(string))
	if err != nil {
		return nil, nil, err
	}

	joinConfig, err = YAMLToJoinConfig(configBytes)
	if err != nil {
		return nil, nil, err
	}

	// ... update some things, like the seeder, the nodename, etc
	joinConfig.Discovery.BootstrapToken.APIServerEndpoint = AddressWithPort(seeder, DefAPIServerPort)
	if nodenameOpt, ok := d.GetOk("nodename"); ok {
		joinConfig.NodeRegistration.Name = nodenameOpt.(string)
	}

	/// ... and serialize again
	configBytes, err = JoinConfigToYAML(joinConfig)
	if err != nil {
		return nil, nil, err
	}

	// ssh.Debug("join config:\n%s\n", configBytes)
	return joinConfig, configBytes, nil
}

// JoinConfigToResourceData updates the `config.join` in the ResourceData with
// the joinConfig provided
func JoinConfigToResourceData(d *schema.ResourceData, joinConfig *kubeadmapi.JoinConfiguration) error {
	joinConfigBytes, err := JoinConfigToYAML(joinConfig)
	if err != nil {
		return fmt.Errorf("could not get a valid 'config' for join'ing: %s", err)
	}

	// update the config.join
	config := d.Get("config").(map[string]interface{})
	config["join"] = ToTerraformSafeString(joinConfigBytes[:])
	if err := d.Set("config", config); err != nil {
		return fmt.Errorf("cannot update config.join")
	}
	return nil
}
