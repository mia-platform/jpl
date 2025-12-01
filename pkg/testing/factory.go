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

package testing

import (
	"fmt"

	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/api/meta/testrestmapper"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/cli-runtime/pkg/resource"
	"k8s.io/client-go/dynamic"
	fakedynamic "k8s.io/client-go/dynamic/fake"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	fakerest "k8s.io/client-go/rest/fake"
	"k8s.io/client-go/restmapper"
	"k8s.io/client-go/tools/clientcmd"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"

	"github.com/mia-platform/jpl/pkg/util"
)

// TestClientFactory extends Factory and provides fake implementation, and affordances for working with kubernetes packages
type TestClientFactory struct {
	util.ClientFactory

	// ClientConfig contains a fake restConfig
	clientConfig *rest.Config

	// Client custom RESTClient implementation to return in UnstructuredClientForMapping and used in KubernetesClientSet
	Client resource.RESTClient
	// UnstructuredClientForMappingFunc custom function to call when UnstructuredClientForMapping is invoked
	UnstructuredClientForMappingFunc resource.FakeClientFunc
	// RESTMapper custom RESTMapper implementation that can be used to augment the supported default types
	RESTMapper meta.RESTMapper

	// FakeDynamicClient custom fake dynamic client implementation to return in DynamicClient
	FakeDynamicClient *fakedynamic.FakeDynamicClient

	testConfigFlags *genericclioptions.TestConfigFlags
}

// NewTestClientFactory return a new TestFactory with a clientgetter that doesn't read from on disk data
func NewTestClientFactory() *TestClientFactory {
	// empty loading rule to avoid reading from local files and env variables
	loadingRules := &clientcmd.ClientConfigLoadingRules{}

	overrides := &clientcmd.ConfigOverrides{ClusterDefaults: clientcmdapi.Cluster{Server: "http://localhost:8080"}}
	clientConfig := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(loadingRules, overrides)
	configFlags := genericclioptions.NewTestConfigFlags().
		WithClientConfig(clientConfig).
		WithRESTMapper(fakeRESTMapper())

	restConfig, err := clientConfig.ClientConfig()
	if err != nil {
		panic(fmt.Sprintf("unable to create a fake restclient config: %v", err))
	}

	return &TestClientFactory{
		ClientFactory:     util.NewFactory(configFlags),
		clientConfig:      restConfig,
		FakeDynamicClient: fakedynamic.NewSimpleDynamicClient(Scheme),
		testConfigFlags:   configFlags,
	}
}

// WithNamespace is used to mention namespace reactively
func (f *TestClientFactory) WithNamespace(ns string) *TestClientFactory {
	f.testConfigFlags.WithNamespace(ns)
	return f
}

// ToRESTConfig reimplement the method for returning a fake clientConfig
func (f *TestClientFactory) ToRESTConfig() (*rest.Config, error) {
	return f.clientConfig, nil
}

// ToRESTMapper reimplement the method for returning a custom mapper
func (f *TestClientFactory) ToRESTMapper() (meta.RESTMapper, error) {
	if f.RESTMapper != nil {
		return f.RESTMapper, nil
	}
	return f.ClientFactory.ToRESTMapper()
}

// DynamicClient reimplement the method for returning a simple fake Dynamic client or a custom one if available
func (f *TestClientFactory) DynamicClient() (dynamic.Interface, error) {
	if f.FakeDynamicClient != nil {
		return f.FakeDynamicClient, nil
	}

	return f.ClientFactory.DynamicClient()
}

// KubernetesClientSet reimplement the method for returning only selected kubernetes clients with fake client
func (f *TestClientFactory) KubernetesClientSet() (kubernetes.Interface, error) {
	fakeClient := f.Client.(*fakerest.RESTClient)
	clientset := kubernetes.NewForConfigOrDie(f.clientConfig)

	clientset.CoreV1().RESTClient().(*rest.RESTClient).Client = fakeClient.Client

	return clientset, nil
}

// UnstructuredClientForMapping reimplement the method for returning the custom client set on the test factory
func (f *TestClientFactory) UnstructuredClientForMapping(mapping *meta.RESTMapping) (resource.RESTClient, error) {
	if f.UnstructuredClientForMappingFunc != nil {
		return f.UnstructuredClientForMappingFunc(mapping.GroupVersionKind.GroupVersion())
	}
	return f.Client, nil
}

// fakeRESTMapper return a test RESTMapper useful for testing out the library
func fakeRESTMapper() meta.RESTMapper {
	groupResources := testGroupResources()
	mapper := restmapper.NewDiscoveryRESTMapper(groupResources)
	mapper = meta.FirstHitRESTMapper{
		MultiRESTMapper: meta.MultiRESTMapper{
			mapper,
			testrestmapper.TestOnlyStaticRESTMapper(Scheme),
		},
	}

	return mapper
}

// testGroupResources return APIGroupResources that we will call during tests
func testGroupResources() []*restmapper.APIGroupResources {
	return []*restmapper.APIGroupResources{
		{
			Group: metav1.APIGroup{
				Versions: []metav1.GroupVersionForDiscovery{
					{Version: "v1"},
				},
				PreferredVersion: metav1.GroupVersionForDiscovery{Version: "v1"},
			},
			VersionedResources: map[string][]metav1.APIResource{
				"v1": {
					{Name: "services", Namespaced: true, Kind: "Service"},
					{Name: "configmaps", Namespaced: true, Kind: "ConfigMap"},
					{Name: "secrets", Namespaced: true, Kind: "Secret"},
					{Name: "namespaces", Namespaced: false, Kind: "Namespace"},
				},
			},
		},
		{
			Group: metav1.APIGroup{
				Name: "apps",
				Versions: []metav1.GroupVersionForDiscovery{
					{Version: "v1"},
				},
				PreferredVersion: metav1.GroupVersionForDiscovery{Version: "v1"},
			},
			VersionedResources: map[string][]metav1.APIResource{
				"v1": {
					{Name: "deployments", Namespaced: true, Kind: "Deployment"},
					{Name: "replicasets", Namespaced: true, Kind: "ReplicaSet"},
				},
			},
		},
		{
			Group: metav1.APIGroup{
				Name: "apiextensions.k8s.io",
				Versions: []metav1.GroupVersionForDiscovery{
					{Version: "v1"},
				},
				PreferredVersion: metav1.GroupVersionForDiscovery{Version: "v1"},
			},
			VersionedResources: map[string][]metav1.APIResource{
				"v1": {
					{Name: "customresourcedefinitions", Namespaced: false, Kind: "CustomResourceDefinition"},
				},
			},
		},
	}
}
