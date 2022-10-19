// Copyright 2020 Mia srl
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
	"path/filepath"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/client-go/rest"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
)

var testEnv *envtest.Environment
var cfg *rest.Config
var clients *K8sClients

var _ = BeforeSuite(func() {

	logf.SetLogger(zap.New(zap.WriteTo(GinkgoWriter), zap.UseDevMode(true)))

	testEnv = &envtest.Environment{
		CRDDirectoryPaths:     []string{filepath.Join("..", "..", "config", "crd", "bases")},
		ErrorIfCRDPathMissing: false,
	}
	var err error
	cfg, err = testEnv.Start()
	Expect(err).NotTo(HaveOccurred())
	Expect(cfg).NotTo(BeNil())

	clients = createRealK8sClients(cfg)
}, 60)

var _ = AfterSuite(func() {
	By("tearing down the test environment")
	if testEnv != nil {
		err := testEnv.Stop()
		Expect(err).NotTo(HaveOccurred())
	}
}, 60)

var _ = Describe("deploy on mock kubernetes", func() {
	deployConfig := DeployConfig{
		ForceDeployWhenNoSemver: true,
		EnsureNamespace:         true,
	}

	Context("apply resources", func() {
		It("creates non existing secret without namespace in metadata", func() {
			err := execDeploy(clients, "test1", []string{"testdata/integration/apply-resources/docker.secret.yaml"}, deployConfig)
			Expect(err).NotTo(HaveOccurred())
			_, err = clients.dynamic.Resource(gvrSecrets).Namespace("test1").
				Get(context.Background(), "docker", metav1.GetOptions{})
			Expect(err).NotTo(HaveOccurred())
		})
		It("creates non existing secret overriding namespace in metadata", func() {
			err := execDeploy(clients, "test2", []string{"testdata/integration/apply-resources/docker-ns.secret.yaml"}, deployConfig)
			Expect(err).NotTo(HaveOccurred())
			_, err = clients.dynamic.Resource(gvrSecrets).Namespace("test2").
				Get(context.Background(), "docker", metav1.GetOptions{})
			Expect(err).NotTo(HaveOccurred())
		})
		It("gives error with no given namespace and no namespace in metadata", func() {
			err := execDeploy(clients, "", []string{"testdata/integration/apply-resources/docker-no-ns.secret.yaml"}, deployConfig)
			Expect(err).To(HaveOccurred())
		})
		It("updates secret", func() {
			err := execDeploy(clients, "test3", []string{"testdata/integration/apply-resources/opaque-1.secret.yaml"}, deployConfig)
			Expect(err).NotTo(HaveOccurred())
			err = execDeploy(clients, "test3", []string{"testdata/integration/apply-resources/opaque-2.secret.yaml"}, deployConfig)
			Expect(err).NotTo(HaveOccurred())
			sec, err := clients.dynamic.Resource(gvrSecrets).Namespace("test3").
				Get(context.Background(), "opaque", metav1.GetOptions{})
			Expect(err).NotTo(HaveOccurred())
			Datakey, _, err := unstructured.NestedString(sec.Object, "data", "key")
			Expect(err).NotTo(HaveOccurred())
			Expect(Datakey).To(Equal("YW5vdGhlcnZhbHVl"))
			By("No annotation last applied for configmap and secrets")
			Expect(sec.GetLabels()[corev1.LastAppliedConfigAnnotation]).To(Equal(""))
		})
		It("updates configmap", func() {
			err := execDeploy(clients, "test3", []string{"testdata/integration/apply-resources/literal-1.configmap.yaml"}, deployConfig)
			Expect(err).NotTo(HaveOccurred())
			err = execDeploy(clients, "test3", []string{"testdata/integration/apply-resources/literal-2.configmap.yaml"}, deployConfig)
			Expect(err).NotTo(HaveOccurred())
			sec, err := clients.dynamic.Resource(gvrConfigMaps).Namespace("test3").
				Get(context.Background(), "literal", metav1.GetOptions{})
			Expect(err).NotTo(HaveOccurred())
			Datakey, _, err := unstructured.NestedString(sec.Object, "data", "unaKey")
			Expect(err).NotTo(HaveOccurred())
			Expect(Datakey).To(Equal("differentValue1"))
			By("No annotation last applied for configmap and secrets")
			Expect(sec.GetLabels()[corev1.LastAppliedConfigAnnotation]).To(Equal(""))
		})
		It("creates and updates depoyment", func() {
			err := execDeploy(clients, "test3", []string{"testdata/integration/apply-resources/test-deployment-1.yaml"}, deployConfig)
			Expect(err).NotTo(HaveOccurred())
			err = execDeploy(clients, "test3", []string{"testdata/integration/apply-resources/test-deployment-2.yaml"}, deployConfig)
			Expect(err).NotTo(HaveOccurred())
			dep, err := clients.dynamic.Resource(gvrDeployments).
				Namespace("test3").
				Get(context.Background(), "test-deployment", metav1.GetOptions{})
			Expect(err).NotTo(HaveOccurred())
			Expect(dep.GetAnnotations()[corev1.LastAppliedConfigAnnotation]).NotTo(Equal(""))
		})
		It("creates job from cronjob", func() {
			err := execDeploy(clients, "test4", []string{"testdata/integration/apply-resources/test-cronjob-1.yaml"}, deployConfig)
			Expect(err).NotTo(HaveOccurred())
			jobList, err := clients.dynamic.Resource(gvrJobs).
				Namespace("test4").
				List(context.Background(), metav1.ListOptions{})
			Expect(err).NotTo(HaveOccurred())
			Expect(jobList.Items[0].GetLabels()["job-name"]).To(ContainSubstring("test-cronjob"))
		})
		It("creates non-namespaced resources", func() {
			err := execDeploy(clients, "test4", []string{"testdata/integration/apply-resources/non-namespaced.yaml"}, deployConfig)
			Expect(err).NotTo(HaveOccurred())
			_, err = clients.dynamic.Resource(gvrCRDs).
				List(context.Background(), metav1.ListOptions{})
			Expect(err).NotTo(HaveOccurred())
		})
		It("replaces non-namespaced resources", func() {
			err := execDeploy(clients, "test4", []string{"testdata/integration/apply-resources/non-namespaced-2.yaml"}, deployConfig)
			Expect(err).NotTo(HaveOccurred())
			_, err = clients.dynamic.Resource(gvrCRDs).
				List(context.Background(), metav1.ListOptions{})
			Expect(err).NotTo(HaveOccurred())
		})
	})
})

// execDeploy combines the deploy function with its helper to apply a configuration
// on the test environment.
// The function implements the 2-step apply to deploy CRDs before anything else
func execDeploy(clients *K8sClients, namespace string, inputPaths []string, deployConfig DeployConfig) error {
	filePaths, err := ExtractYAMLFiles(inputPaths)
	CheckError(err, "Error extracting yaml files")

	crds, resources, err := MakeResources(filePaths, namespace, RealSupportedResourcesGetter{}, clients)
	if err != nil {
		return fmt.Errorf("fails to make resources: %w", err)
	}

	// deploy CRDs first (simplified example of 2-step deployment w/o checks on CRDs' status)...
	err = Deploy(clients, namespace, crds, deployConfig, defaultApplyResource)
	if err != nil {
		return fmt.Errorf("fails to deploy crds: %w", err)
	}

	// ...and then all the remaining resources
	err = Deploy(clients, namespace, resources, deployConfig, defaultApplyResource)
	if err != nil {
		return fmt.Errorf("fails to deploy resources: %w", err)
	}

	return Cleanup(clients, namespace, resources)
}
