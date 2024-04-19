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

package util

import (
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/cli-runtime/pkg/resource"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

// ClientFactory provides abstractions that allow to change the certain functions in certain cases, like testing
type ClientFactory interface {
	genericclioptions.RESTClientGetter

	// DynamicClient returns a dynamic client ready for use
	DynamicClient() (dynamic.Interface, error)

	// UnstructuredClientForMapping return a RESTClient that can be used for the Unstructured object described by mapping
	UnstructuredClientForMapping(mapping *meta.RESTMapping) (resource.RESTClient, error)

	// KubernetesClientSet gives you back an external clientset
	KubernetesClientSet() (kubernetes.Interface, error)
}

// NewFactory return a CleintFactory implementation
func NewFactory(clientGetter genericclioptions.RESTClientGetter) ClientFactory {
	return &factoryImplementation{
		delegate: clientGetter,
	}
}

type factoryImplementation struct {
	delegate genericclioptions.RESTClientGetter
}

// ToDiscoveryClient implement genericclioptions.RESTClientGetter
func (f *factoryImplementation) ToDiscoveryClient() (discovery.CachedDiscoveryInterface, error) {
	return f.delegate.ToDiscoveryClient()
}

// ToRESTConfig implement genericclioptions.RESTClientGetter
func (f *factoryImplementation) ToRESTConfig() (*rest.Config, error) {
	return f.delegate.ToRESTConfig()
}

// ToRESTMapper implement genericclioptions.RESTClientGetter
func (f *factoryImplementation) ToRESTMapper() (meta.RESTMapper, error) {
	return f.delegate.ToRESTMapper()
}

// ToRawKubeConfigLoader implement genericclioptions.RESTClientGetter
func (f *factoryImplementation) ToRawKubeConfigLoader() clientcmd.ClientConfig {
	return f.delegate.ToRawKubeConfigLoader()
}

// DynamicClient returns a dynamic client ready for use
func (f *factoryImplementation) DynamicClient() (dynamic.Interface, error) {
	clientConfig, err := f.ToRESTConfig()
	if err != nil {
		return nil, err
	}

	return dynamic.NewForConfig(clientConfig)
}

// UnstructuredClientForMapping return a RESTClient that can be used for the Unstructured object described by mapping
func (f *factoryImplementation) UnstructuredClientForMapping(mapping *meta.RESTMapping) (resource.RESTClient, error) {
	cfg, err := f.delegate.ToRESTConfig()

	if err != nil {
		return nil, err
	}
	if err := rest.SetKubernetesDefaults(cfg); err != nil {
		return nil, err
	}
	cfg.APIPath = "/apis"
	if mapping.GroupVersionKind.Group == corev1.GroupName {
		cfg.APIPath = "/api"
	}
	gv := mapping.GroupVersionKind.GroupVersion()
	cfg.ContentConfig = resource.UnstructuredPlusDefaultContentConfig()
	cfg.GroupVersion = &gv
	return rest.RESTClientFor(cfg)
}

// KubernetesClientSet gives you back an external clientset
func (f *factoryImplementation) KubernetesClientSet() (kubernetes.Interface, error) {
	clientConfig, err := f.ToRESTConfig()
	if err != nil {
		return nil, err
	}

	return kubernetes.NewForConfig(clientConfig)
}
