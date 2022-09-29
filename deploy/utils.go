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
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/Masterminds/semver"
	"github.com/spf13/afero"
	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	batchv1beta1 "k8s.io/api/batch/v1beta1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/cli-runtime/pkg/printers"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/discovery/cached/memory"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/restmapper"
	k8syaml "sigs.k8s.io/yaml"
)

const (
	StdinToken         = "-"
	resourceSecretName = "resources-deployed"
	resourceField      = "resources"
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

type ResourceList struct {
	Kind      *schema.GroupVersionKind `json:"kind"`
	Resources []string                 `json:"resources"`
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

// Options global option struct
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

// NewOptions create a new options struct
func NewOptions() *Options {
	options := &Options{
		Kubeconfig: os.Getenv("KUBECONFIG"),
	}

	// bind to kubernetes config flags
	options.Config = &genericclioptions.ConfigFlags{
		CAFile:       &options.CertificateAuthority,
		CertFile:     &options.ClientCertificate,
		KeyFile:      &options.ClientKey,
		ClusterName:  &options.Cluster,
		Context:      &options.Context,
		KubeConfig:   &options.Kubeconfig,
		Insecure:     &options.InsecureSkipTLSVerify,
		Namespace:    &options.Namespace,
		APIServer:    &options.Server,
		BearerToken:  &options.Token,
		AuthInfoName: &options.User,
	}

	return options
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
		res, err := NewResourcesFromFile(path, namespace)
		if err != nil {
			return nil, err
		}
		resources = append(resources, res...)
	}

	resources = SortResourcesByKind(resources, nil)
	return resources, nil
}

// NewResourcesFromFile creates new Resources from a file at `filepath`
// Supports multiple documents inside a single file
func NewResourcesFromFile(filepath, namespace string) ([]Resource, error) {
	var stream []byte
	var err error

	if filepath == StdinToken {
		stream, err = io.ReadAll(os.Stdin)
	} else {
		stream, err = os.ReadFile(filepath)
	}
	if err != nil {
		return nil, err
	}

	return createResourcesFromBuffer(stream, namespace, filepath)
}

// NewResourcesFromBuffer exposes the createResourcesFromBuffer function
// setting the filepath to "buffer"
func NewResourcesFromBuffer(stream []byte, namespace string) ([]Resource, error) {
	return createResourcesFromBuffer(stream, namespace, "buffer")
}

// createResourcesFromBuffer creates new Resources from a byte stream
// Supports multiple resources divided by `---`
func createResourcesFromBuffer(stream []byte, namespace string, filepath string) ([]Resource, error) {
	var resources []Resource
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

		if namespace != "" {
			u.SetNamespace(namespace)
		}

		resources = append(resources,
			Resource{
				Filepath:         filepath,
				GroupVersionKind: &gvk,
				Object:           u,
			})
	}
	resources = SortResourcesByKind(resources, nil)
	return resources, nil
}

// makeResourceMap groups the resources list by kind and embeds them in a `ResourceList` struct
func makeResourceMap(resources []Resource) map[string]*ResourceList {
	res := make(map[string]*ResourceList)

	for _, r := range resources {
		if _, ok := res[r.GroupVersionKind.Kind]; !ok {
			res[r.GroupVersionKind.Kind] = &ResourceList{
				Kind:      r.GroupVersionKind,
				Resources: []string{},
			}
		}
		res[r.GroupVersionKind.Kind].Resources = append(res[r.GroupVersionKind.Kind].Resources, r.Object.GetName())
	}

	return res
}

// getOldResourceMap fetches the last set of resources deployed into the namespace from
// `resourceSecretName` secret.
func getOldResourceMap(clients *K8sClients, namespace string) (map[string]*ResourceList, error) {
	var secret corev1.Secret
	secretUnstr, err := clients.dynamic.Resource(gvrSecrets).
		Namespace(namespace).Get(context.Background(), resourceSecretName, metav1.GetOptions{})

	if err != nil {
		if apierrors.IsNotFound(err) {
			return map[string]*ResourceList{}, nil
		}
		return nil, err
	}

	err = runtime.DefaultUnstructuredConverter.
		FromUnstructured(secretUnstr.Object, &secret)
	if err != nil {
		return nil, err
	}

	res := make(map[string]*ResourceList)

	resources := secret.Data[resourceField]
	if strings.Contains(string(resources), "\"Mapping\":{") {
		res, err = convertSecretFormat(resources)
	} else {
		err = json.Unmarshal(resources, &res)
	}
	if err != nil {
		return nil, err
	}

	if len(res) == 0 {
		return nil, errors.New("resource field is empty")
	}

	return res, nil
}

// deletedResources returns the resources not contained in the last deploy
func deletedResources(actual, old map[string]*ResourceList) map[string]*ResourceList {
	res := make(map[string]*ResourceList)

	// get diff on already existing resources, the new ones
	// are added with the new secret.
	for key := range old {
		if _, ok := res[key]; !ok {
			res[key] = &ResourceList{
				Kind: old[key].Kind,
			}
		}

		if _, ok := actual[key]; ok {
			res[key].Resources = diffResourceArray(actual[key].Resources, old[key].Resources)
		} else {
			res[key].Resources = old[key].Resources
		}
	}

	// Remove entries with empty diff
	for kind, resourceGroup := range res {
		if len(resourceGroup.Resources) == 0 {
			delete(res, kind)
		}
	}

	return res
}

// prune deletes the given resources on the cluster
func prune(clients *K8sClients, namespace string, resourceGroup *ResourceList) error {
	for _, res := range resourceGroup.Resources {
		fmt.Printf("Deleting: %v %v\n", resourceGroup.Kind.Kind, res)

		gvr, err := FromGVKtoGVR(clients.discovery, *resourceGroup.Kind)
		if err != nil {
			return err
		}
		_, err = clients.dynamic.Resource(gvr).Namespace(namespace).
			Get(context.Background(), res, metav1.GetOptions{})
		if err != nil {
			if apierrors.IsNotFound(err) {
				fmt.Printf("already not present on cluster\n")
				continue
			} else {
				return err
			}
		}
		err = clients.dynamic.Resource(gvr).Namespace(namespace).
			Delete(context.Background(), res, metav1.DeleteOptions{})

		if err != nil && !apierrors.IsNotFound(err) {
			return err
		}
	}
	return nil
}

// updateResourceSecret updates the resource secret with the given resource map
func updateResourceSecret(dynamic dynamic.Interface, namespace string, resources map[string]*ResourceList) error {
	secretContent, err := json.Marshal(resources)
	if err != nil {
		return err
	}
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      resourceSecretName,
			Namespace: namespace,
		},
		TypeMeta: metav1.TypeMeta{Kind: "Secret", APIVersion: "v1"},
		Data:     map[string][]byte{"resources": secretContent},
	}

	unstr, err := fromRuntimeObjtoUnstruct(secret, secret.GroupVersionKind())
	if err != nil {
		return err
	}

	if _, err = dynamic.Resource(gvrSecrets).
		Namespace(unstr.GetNamespace()).
		Create(context.Background(), unstr, metav1.CreateOptions{}); err != nil {
		if apierrors.IsAlreadyExists(err) {
			_, err = dynamic.Resource(gvrSecrets).
				Namespace(unstr.GetNamespace()).
				Update(context.Background(),
					unstr,
					metav1.UpdateOptions{})
			if err != nil {
				return err
			}
		} else {
			return err
		}
	}
	return nil
}

// convertSecretFormat converts the secret format in the compatible one
func convertSecretFormat(resources []byte) (map[string]*ResourceList, error) {
	type oldResourceList struct {
		Kind      string `json:"kind"`
		Mapping   schema.GroupVersionResource
		Resources []string `json:"resources"`
	}

	oldres := make(map[string]*oldResourceList)
	err := json.Unmarshal(resources, &oldres)
	if err != nil {
		return nil, err
	}

	res := make(map[string]*ResourceList)

	for k, v := range oldres {
		res[k] = &ResourceList{
			Kind: &schema.GroupVersionKind{
				Group:   v.Mapping.Group,
				Version: v.Mapping.Version,
				Kind:    k,
			},
			Resources: v.Resources}
	}
	return res, nil
}

// fromRuntimeObjtoUnstruct converts runtime object to unstructured.Unstructured
func fromRuntimeObjtoUnstruct(obj runtime.Object, gvk schema.GroupVersionKind) (*unstructured.Unstructured, error) {
	currentObj := &unstructured.Unstructured{}
	currentObj.SetGroupVersionKind(gvk)
	interfCurrentObj, err := runtime.DefaultUnstructuredConverter.ToUnstructured(&obj)
	if err != nil {
		return nil, err
	}
	return &unstructured.Unstructured{
		Object: interfCurrentObj,
	}, nil
}

// diffResourceArray returns the old values missing in the new slice
func diffResourceArray(actual, old []string) []string {
	res := []string{}

	for _, oValue := range old {
		if !contains(actual, oValue) {
			res = append(res, oValue)
		}
	}

	return res
}

// contains takes a string slice and search for an element in it.
func contains(res []string, s string) bool {
	for _, item := range res {
		if item == s {
			return true
		}
	}
	return false
}

// CheckError default error handling function
func CheckError(err error, msg string) {
	if err != nil {
		log.Fatal(err, msg)
	}
}

// ExtractYAMLFiles return array of YAML filenames from array of files and directories
func ExtractYAMLFiles(paths []string) ([]string, error) {
	if len(paths) == 1 && paths[0] == StdinToken {
		return []string{StdinToken}, nil
	}

	fileNames := []string{}

	// Extract files from directories
	for _, path := range paths {
		// get absolute path for good measure
		globalPath, err := filepath.Abs(path)
		if err != nil {
			return nil, err
		}

		pathIsDirectory, err := fs.IsDir(globalPath)
		if err != nil {
			fmt.Printf("WARN: can't read input file at path %s\n", globalPath)
			continue
		}

		if pathIsDirectory {
			pathsInDirectory, err := extractYAMLFromDir(globalPath)
			if err != nil {
				return nil, err
			}

			fileNames = append(fileNames, pathsInDirectory...)
		} else if isYAMLFile(globalPath) {
			fileNames = append(fileNames, globalPath)
		}
	}
	return fileNames, nil
}

// extractYAMLFromDir extracts from a directory the global path to YAML files contained in it.
// This function does not look into subdirs.
func extractYAMLFromDir(directoryPath string) ([]string, error) {
	filesPath := []string{}

	files, err := fs.ReadDir(directoryPath)
	if err != nil {
		return nil, err
	}

	for _, path := range files {
		if !path.IsDir() && isYAMLFile(path.Name()) {
			filesPath = append(filesPath, filepath.Join(directoryPath, path.Name()))
		}
	}

	return filesPath, nil
}

// isYAMLFile this function return true if the path contain a known YAML extension
func isYAMLFile(path string) bool {
	fileExtension := filepath.Ext(path)
	return fileExtension == ".yaml" || fileExtension == ".yml"
}

// WriteYamlsToDisk marshals and writes kubernetes runtime objects to YAML file
func WriteYamlsToDisk(objs map[string]runtime.Object, outputDirectory string) {
	printer := &printers.YAMLPrinter{}
	for yamlName, obj := range objs {
		fileName := outputDirectory + "/" + yamlName + ".yaml"
		file, err := CreateFile(fileName)
		CheckError(err, "")
		CheckError(printer.PrintObj(obj, file), "")
	}
}

// ReadFile read a file from the file system
func ReadFile(filename string) ([]byte, error) {
	return fs.ReadFile(filename)
}

// MkdirAll create a folder
func MkdirAll(name string) error {
	userFolderReadOnlyPerm := 0755
	return fs.MkdirAll(name, os.FileMode(userFolderReadOnlyPerm))
}

// RemoveAll removes a directory path and any children it contains.
func RemoveAll(path string) error {
	return fs.RemoveAll(path)
}

// CreateFile create a new file in path
func CreateFile(path string) (afero.File, error) {
	return fs.Create(path)
}

// WriteFile write data to file
func WriteFile(filename string, data []byte) error {
	userWriteOnlyFilePerm := 0644
	return fs.WriteFile(filename, data, os.FileMode(userWriteOnlyFilePerm))
}
