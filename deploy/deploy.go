// Copyright 2022 Mia srl
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

package jpl

import (
	"fmt"

	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/dynamic"
)

// DeployConfig are all the specific configurations used in deploy phase
type DeployConfig struct {
	DeployType              string
	ForceDeployWhenNoSemver bool
	EnsureNamespace         bool
}

// Options global option for the cli that can be passed to all commands
type Options struct {
	Config *genericclioptions.ConfigFlags

	CertificateAuthority  string
	ClientCertificate     string
	ClientKey             string
	Cluster               string
	Context               string
	Kubeconfig            string
	InsecureSkipTLSVerify bool
	Namespace             string
	Server                string
	Token                 string
	User                  string
}

// Deploy ensures namespace existence and applies the resources to the cluster
func Deploy(clients *K8sClients, namespace string, resources []Resource, deployConfig DeployConfig, apply ApplyFunction) error {

	// for each resource ensure namespace if a namespace is not passed to the function ensure namespace in the resource, gives error
	// on no namespace passed to the function and no namespace in yaml
	// The namespace given to the function overrides yaml namespace
	for _, res := range resources {
		if namespace == "" {
			resourceNamespace := res.Object.GetNamespace()
			if resourceNamespace != "" && deployConfig.EnsureNamespace {
				if err := ensureNamespaceExistence(clients, resourceNamespace); err != nil {
					return err
				}
			} else if resourceNamespace == "" {
				return fmt.Errorf("no namespace passed and no namespace in resource: %s %s", res.GroupVersionKind.Kind, res.Object.GetName())
			}
		} else {
			res.Object.SetNamespace(namespace)
		}
	}

	if namespace != "" && deployConfig.EnsureNamespace {
		if err := ensureNamespaceExistence(clients, namespace); err != nil {
			return err
		}
	}

	// apply the resources
	for _, res := range resources {
		err := apply(clients, res, deployConfig)
		if err != nil {
			return err
		}
	}
	return nil
}

// InitK8sClients returns an initialized K8sClients struct to be used
// for the deployment process
func InitK8sClients(inputPaths []string, deployConfig DeployConfig, opts *Options) *K8sClients {
	restConfig, err := opts.Config.ToRESTConfig()
	CheckError(err, "")

	// The following two options manage client-side throttling to decrease pressure on api-server
	// Kubectl sets 300 bursts 50.0 QPS:
	// https://github.com/kubernetes/kubectl/blob/0862c57c87184432986c85674a237737dabc53fa/pkg/cmd/cmd.go#L92
	restConfig.QPS = 500.0
	restConfig.Burst = 500

	return &K8sClients{
		dynamic:   dynamic.NewForConfigOrDie(restConfig),
		discovery: discovery.NewDiscoveryClientForConfigOrDie(restConfig),
	}
}

// Cleanup removes the resources no longer deployed and updates
// the secret in the cluster with the updated set of resources
func Cleanup(clients *K8sClients, namespace string, resources []Resource) error {
	actual := makeResourceMap(resources)

	old, err := getOldResourceMap(clients, namespace)
	if err != nil {
		return err
	}

	// Prune only if it is not the first release
	if len(old) != 0 {
		deleteMap := deletedResources(actual, old)

		for _, resourceGroup := range deleteMap {
			err = prune(clients, namespace, resourceGroup)
			if err != nil {
				return err
			}
		}
	}
	err = updateResourceSecret(clients.dynamic, namespace, actual)
	return err
}
