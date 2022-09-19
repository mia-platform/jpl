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
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io/ioutil"
	"os"
	"regexp"
	"strings"

	"github.com/Masterminds/semver"
	"github.com/spf13/afero"
	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	batchv1beta1 "k8s.io/api/batch/v1beta1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/discovery/cached/memory"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/restmapper"
	k8syaml "sigs.k8s.io/yaml"
)

const (
	StdinToken string = "-"
)

var fs = &afero.Afero{Fs: afero.NewOsFs()}

var (
	gvrSecrets     = schema.GroupVersionResource{Group: "", Version: "v1", Resource: "secrets"}
	gvrNamespaces  = schema.GroupVersionResource{Group: "", Version: "v1", Resource: "namespaces"}
	gvrConfigMaps  = schema.GroupVersionResource{Group: "", Version: "v1", Resource: "configmaps"}
	gvrDeployments = schema.GroupVersionResource{Group: "apps", Version: "v1", Resource: "deployments"}
	gvrJobs        = schema.GroupVersionResource{Group: batchv1.SchemeGroupVersion.Group, Version: batchv1.SchemeGroupVersion.Version, Resource: "jobs"}
)

type Resource struct {
	Filepath         string
	GroupVersionKind *schema.GroupVersionKind
	Object           unstructured.Unstructured
}

type K8sClients struct {
	dynamic   dynamic.Interface
	discovery discovery.DiscoveryInterface
}

func FromGVKtoGVR(discoveryClient discovery.DiscoveryInterface, gvk schema.GroupVersionKind) (schema.GroupVersionResource, error) {
	mapper := restmapper.NewDeferredDiscoveryRESTMapper(memory.NewMemCacheClient(discoveryClient))
	a, err := mapper.RESTMapping(gvk.GroupKind(), gvk.Version)
	if err != nil {
		return schema.GroupVersionResource{}, err
	}
	return a.Resource, nil
}

// GetMiaAnnotation is used to get an annotation name following a pattern used in mia-platform
func GetMiaAnnotation(name string) string {
	return fmt.Sprintf("mia-platform.eu/%s", strings.ReplaceAll(name, " ", "-"))
}

// GetChecksum is used to calculate a checksum using an array of bytes
func GetChecksum(content []byte) string {
	checkSum := sha256.Sum256(content)
	return hex.EncodeToString(checkSum[:])
}

// IsNotUsingSemver is used to check if a resoure is following semver or not
func IsNotUsingSemver(target *Resource) (bool, error) {
	var containers []corev1.Container
	var err error
	switch target.GroupVersionKind.Kind {
	case "Deployment":
		var desiredDeployment appsv1.Deployment
		err = runtime.DefaultUnstructuredConverter.
			FromUnstructured(target.Object.Object, &desiredDeployment)
		containers = desiredDeployment.Spec.Template.Spec.Containers
	case "CronJob":
		var desiredCronJob batchv1beta1.CronJob
		err = runtime.DefaultUnstructuredConverter.
			FromUnstructured(target.Object.Object, &desiredCronJob)
		containers = desiredCronJob.Spec.JobTemplate.Spec.Template.Spec.Containers
	}
	if err != nil {
		return false, err
	}

	for _, container := range containers {
		if !strings.Contains(container.Image, ":") {
			return true, nil
		}
		imageVersion := strings.Split(container.Image, ":")[1]
		if _, err := semver.NewVersion(imageVersion); err != nil {
			return true, nil
		}
	}
	return false, nil
}

// MakeResources takes a filepath/buffer and returns the Kubernetes resources in them
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
