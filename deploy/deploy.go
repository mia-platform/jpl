// Copyright Mia srl
// SPDX-License-Identifier: Apache-2.0
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
	"context"
	"fmt"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/client-go/discovery"
	discoveryFake "k8s.io/client-go/discovery/fake"
	"k8s.io/client-go/dynamic"
	dynamicFake "k8s.io/client-go/dynamic/fake"
	"k8s.io/client-go/rest"
)

// DeployConfig are all the specific configurations used in deploy phase
type DeployConfig struct {
	DeployType              string
	ForceDeployWhenNoSemver bool
	EnsureNamespace         bool
}

// Deploy ensures namespace existence and applies the resources to the cluster
func Deploy(clients *K8sClients, namespace string, resources []Resource, deployConfig DeployConfig, supportedResourcesGetter SupportedResourcesGetter, apply ApplyFunction) error {
	if namespace != "" && deployConfig.EnsureNamespace {
		if err := ensureNamespaceExistence(clients, namespace); err != nil {
			return fmt.Errorf("error ensuring namespace existence for namespace %s: %w", namespace, err)
		}
	}

	validResources, unknownResources, err := validateResourcesOnCluster(resources, namespace, supportedResourcesGetter, clients)
	if err != nil {
		return err
	}

	if len(unknownResources) > 0 {
		return fmt.Errorf("trying to deploy unknown resources in the cluster")
	}

	// apply the resources
	for _, res := range validResources {
		err := apply(clients, res, deployConfig)
		if err != nil {
			return fmt.Errorf("error applying resource %+v: %w", res, err)
		}
	}
	return nil
}

// InitRealK8sClients initializes a K8sClients struct from given CLI options,
// to be used for the deployment process
func InitRealK8sClients(opts *Options) *K8sClients {
	restConfig, err := opts.Config.ToRESTConfig()
	CheckError(err, "")

	// The following two options manage client-side throttling to decrease pressure on api-server
	// Kubectl sets 300 bursts 50.0 QPS:
	// https://github.com/kubernetes/kubectl/blob/0862c57c87184432986c85674a237737dabc53fa/pkg/cmd/cmd.go#L92
	restConfig.QPS = 500.0
	restConfig.Burst = 500

	return createRealK8sClients(restConfig)
}

// createRealK8sClients returns an initialized K8sClients struct,
// given a REST config
func createRealK8sClients(cfg *rest.Config) *K8sClients {
	clients := &K8sClients{
		dynamic:   dynamic.NewForConfigOrDie(cfg),
		discovery: discovery.NewDiscoveryClientForConfigOrDie(cfg),
	}
	return clients
}

// FakeK8sClients returns a struct of fake k8s clients for testing purposes
func FakeK8sClients() *K8sClients {
	return &K8sClients{
		dynamic:   &dynamicFake.FakeDynamicClient{},
		discovery: &discoveryFake.FakeDiscovery{},
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

// ensureNamespaceExistence verifies whether the given namespace already exists
// on the cluster, and creates it if missing
func ensureNamespaceExistence(clients *K8sClients, namespace string) error {
	ns := &unstructured.Unstructured{}
	ns.SetUnstructuredContent(map[string]interface{}{
		"apiVersion": "v1",
		"kind":       "Namespace",
		"metadata": map[string]interface{}{
			"name": namespace,
		},
	})

	fmt.Printf("Creating namespace %s\n", namespace)
	if _, err := clients.dynamic.Resource(gvrNamespaces).Create(context.Background(), ns, metav1.CreateOptions{}); err != nil && !apierrors.IsAlreadyExists(err) {
		return err
	}

	return nil
}
