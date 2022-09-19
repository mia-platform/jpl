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

package deploy

import (
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"regexp"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	k8syaml "sigs.k8s.io/yaml"
)

type DeployConfig struct {
	DeployType              string
	ForceDeployWhenNoSemver bool
	EnsureNamespace         bool
}

//MakeResources takes a filepath/buffer and returns the Kubernetes resources in them
func MakeResources(filePaths []string, namespace string) ([]Resource, error) {
	resources := []Resource{}
	for _, path := range filePaths {

		res, err := NewResources(path, namespace)
		if err != nil {
			return nil, err
		}
		resources = append(resources, res...)
	}

	resources = SortResourcesByKind(resources, nil)
	return resources, nil
}

// NewResources creates new Resources from a file at `filepath`
// support multiple documents inside a single file
func NewResources(filepath, namespace string) ([]Resource, error) {
	var resources []Resource
	var stream []byte
	var err error

	if filepath == StdinToken {
		stream, err = ioutil.ReadAll(os.Stdin)
	} else {
		stream, err = fs.ReadFile(filepath)
	}
	if err != nil {
		return nil, err
	}

	// split resources on --- yaml document delimiter
	re := regexp.MustCompile(`\n---\n`)
	for _, resourceYAML := range re.Split(string(stream), -1) {

		if len(resourceYAML) == 0 {
			continue
		}

		u := unstructured.Unstructured{Object: map[string]interface{}{}}
		if err := k8syaml.Unmarshal([]byte(resourceYAML), &u.Object); err != nil {
			return nil, fmt.Errorf("resource %s: %s", filepath, err)
		}
		gvk := u.GroupVersionKind()
		u.SetNamespace(namespace)

		resources = append(resources,
			Resource{
				Filepath:         filepath,
				GroupVersionKind: &gvk,
				Object:           u,
			})
	}
	return resources, nil
}

func Deploy(clients *K8sClients, namespace string, resources []Resource, deployConfig DeployConfig, apply applyFunction) error {

	// for each resource ensure namespace if a namespace is not passed to mlp ensure namespace in the resource, gives error
	// on no namespace passed to mlp and no namespace in yaml
	// The namespace given to mlp overrides yaml namespace
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
