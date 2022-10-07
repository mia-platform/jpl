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
	"context"
	"fmt"
	"sync"

	apiextv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	apiextv1beta1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1beta1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
)

// DeployConfig are all the specific configurations used in deploy phase
type DeployConfig struct {
	DeployType              string
	ForceDeployWhenNoSemver bool
	EnsureNamespace         bool
}

// Deploy ensures namespace existence and applies the resources to the cluster
func Deploy(clients *K8sClients, namespace string, resources []Resource, deployConfig DeployConfig, apply ApplyFunction) error {
	// for each resource ensure namespace if a namespace is not passed to the function ensure namespace in the resource, gives error
	// on no namespace passed to the function and no namespace in yaml
	// The namespace given to the function overrides yaml namespace
	for _, res := range resources {
		if res.Namespaced {
			if namespace == "" {
				resourceNamespace := res.Object.GetNamespace()
				if resourceNamespace != "" && deployConfig.EnsureNamespace {
					if err := ensureNamespaceExistence(clients, resourceNamespace); err != nil {
						return fmt.Errorf("error ensuring namespace existence for namespace %s: %w", resourceNamespace, err)
					}
				} else if resourceNamespace == "" {
					return fmt.Errorf("no namespace passed and no namespace in resource: %s %s", res.GroupVersionKind.Kind, res.Object.GetName())
				}
			} else {
				res.Object.SetNamespace(namespace)
			}
		}
	}

	if namespace != "" && deployConfig.EnsureNamespace {
		if err := ensureNamespaceExistence(clients, namespace); err != nil {
			return fmt.Errorf("error ensuring namespace existence for namespace %s: %w", namespace, err)
		}
	}

	// apply the resources
	for _, res := range resources {
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

	return CreateK8sClients(restConfig)
}

var addToScheme sync.Once

// CreateK8sClients returns an initialized K8sClients struct,
// given a REST config
func CreateK8sClients(cfg *rest.Config) *K8sClients {
	// Add CRDs to the scheme. They are missing by default.
	addToScheme.Do(func() {
		if err := apiextv1.AddToScheme(scheme.Scheme); err != nil {
			// This should never happen.
			panic(err)
		}
		if err := apiextv1beta1.AddToScheme(scheme.Scheme); err != nil {
			panic(err)
		}
	})
	clients := &K8sClients{
		dynamic:   dynamic.NewForConfigOrDie(cfg),
		discovery: discovery.NewDiscoveryClientForConfigOrDie(cfg),
	}
	return clients
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
