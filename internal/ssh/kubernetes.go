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

package ssh

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"strings"
)

const (
	// DefAdminKubeconfig is the default "admin.conf" file path
	DefAdminKubeconfig = "/etc/kubernetes/admin.conf"
)

// Manifest represents a manifest, that can be a local file name, a remote URL or inlined
type Manifest struct {
	Path   string
	URL    string
	Inline string
}

// NewManifest creates a new manifest
func NewManifest(m string) Manifest {
	if isValidURL(m) {
		return Manifest{URL: m}
	}
	if LocalFileExists(m) {
		return Manifest{Path: m}
	}
	return Manifest{Inline: m}
}

// IsEmpty returns True iff the manifest does not have a URL, manifest or inline
func (m Manifest) IsEmpty() bool {
	if m.Inline == "" && m.Path == "" && m.URL == "" {
		return true
	}
	return false
}

// ReplaceConfig performs replacements in all the fields in the manifest
func (m *Manifest) ReplaceConfig(config map[string]interface{}) error {
	switch {
	case m.Inline != "":
		replaced, err := ReplaceInTemplate(m.Inline, config)
		if err != nil {
			return err
		}
		m.Inline = replaced
	case m.Path != "":
		replaced, err := ReplaceInTemplate(m.Path, config)
		if err != nil {
			return err
		}
		m.Path = replaced
	case m.URL != "":
		replaced, err := ReplaceInTemplate(m.URL, config)
		if err != nil {
			return err
		}
		m.URL = replaced
	}
	return nil
}

// isValidURL tests a string to determine if it is a url or not.
func isValidURL(toTest string) bool {
	_, err := url.ParseRequestURI(toTest)
	if err != nil {
		return false
	}
	return true
}

/////////////////////////////////////////////////////////////////////////////////

//
// nodes info
//

var (
	// ErrParseNodename is an error parsig the nodename
	ErrParseNodename = errors.New("error parsing node info line")
)

// KubeNode is a node info in Kubernetes
type KubeNode struct {
	Nodename string
	IP       string
	Hostname string
}

func (kn KubeNode) String() string {
	name := kn.Nodename
	if kn.Hostname != kn.Nodename {
		name = fmt.Sprintf("%s/%s", kn.Nodename, kn.Hostname)
	}
	if kn.IP != "" {
		name = fmt.Sprintf("%s(%s)", name, kn.IP)
	}
	return name
}

// IsEmpty returns True iff the KubeNode info is empty
func (kn KubeNode) IsEmpty() bool {
	return kn.Nodename == "" && kn.IP == "" && kn.Hostname == ""
}

/////////////////////////////////////////////////////////////////////////////////

// DoRemoteKubectl runs a remote kubectl command in a remote machine
// it takes care about uploading a valid kubeconfig file if not present in the remote machine
func DoRemoteKubectl(kubectl string, kubeconfig string, args ...string) Action {
	argsStr := strings.Join(args, " ")

	return DoIfElse(
		// note on the cache: if present, the "admin.conf" is never deleted,
		//                    so it is safe to store the result in the cache
		CheckFileExistsOnce(DefAdminKubeconfig),
		ActionList{
			DoExec(fmt.Sprintf("kubectl --kubeconfig=%s %s", DefAdminKubeconfig, argsStr)),
		},
		ActionList{
			ActionFunc(func(context.Context) Action {
				// delay the kubeconfig check:
				if kubeconfig == "" {
					return ActionError("no kubeconfig provided, and no remote admin.conf found")
				}

				// upload the local kubeconfig to some temporary remote file
				remoteKubeconfig, err := GetTempFilename()
				if err != nil {
					return ActionError(fmt.Sprintf("Could not create temporary file: %s", err))
				}

				return DoRetry(
					Retry{Times: 3},
					DoWithCleanup(
						ActionList{
							DoUploadFileToFile(kubeconfig, remoteKubeconfig),
							DoExec(fmt.Sprintf("%s --kubeconfig=%s %s", kubectl, remoteKubeconfig, argsStr)),
						},
						DoTry(DoDeleteFile(remoteKubeconfig))))
			}),
		})
}

// DoRemoteKubectlApply applies some manifests with a remote kubectl
// manifests can be 1) a local file 2) a URL 3) in a string
func DoRemoteKubectlApply(kubectl string, kubeconfig string, manifests []Manifest) Action {
	actions := ActionList{}

	for _, manifest := range manifests {
		switch {
		case manifest.Inline != "":
			actions = append(actions,
				ActionFunc(func(context.Context) Action {
					remoteManifest, err := GetTempFilename()
					if err != nil {
						return ActionError(fmt.Sprintf("Could not create temporary file: %s", err))
					}
					return DoWithCleanup(
						ActionList{
							DoUploadBytesToFile([]byte(manifest.Inline), remoteManifest),
							DoRemoteKubectl(kubectl, kubeconfig, "apply", "-f", remoteManifest),
						}, ActionList{
							DoTry(DoDeleteFile(remoteManifest)),
						})
				}))

		case manifest.Path != "":
			actions = append(actions,
				ActionFunc(func(context.Context) Action {
					// it is a file: upload the file to a temporary, remote file and then `kubectl apply -f` it
					remoteManifest, err := GetTempFilename()
					if err != nil {
						return ActionError(fmt.Sprintf("Could not create temporary file: %s", err))
					}
					return DoWithCleanup(
						ActionList{
							DoUploadFileToFile(manifest.Path, remoteManifest),
							DoRemoteKubectl(kubectl, kubeconfig, "apply", "-f", remoteManifest),
						}, ActionList{
							DoTry(DoDeleteFile(remoteManifest)),
						})
				}))

		case manifest.URL != "":
			// it is an URL: just run the `kubectl apply`
			actions = append(actions, DoRemoteKubectl(kubectl, kubeconfig, "apply", "-f", manifest.URL))
		}
	}

	return actions
}
