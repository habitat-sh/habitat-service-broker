// Copyright (c) 2018 Chef Software Inc. and/or applicable contributors
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

package framework

import (
	"fmt"
	"path/filepath"

	catalogclientset "github.com/kubernetes-incubator/service-catalog/pkg/client/clientset_generated/clientset"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/helm/pkg/helm"
	"k8s.io/helm/pkg/helm/portforwarder"
	"k8s.io/helm/pkg/kube"
)

const (
	TestNs         = "testing-habitat-broker"
	BrokerChartDir = "../../charts/habitat-service-broker"
	ReleaseName    = "habitat-service-broker"
)

type Framework struct {
	KubeClient       kubernetes.Interface
	HelmClient       *helm.Client
	CatalogClientset *catalogclientset.Clientset
	ExternalIP       string
	TillerTunnel     *kube.Tunnel
}

func (f *Framework) TearDown() error {
	opts := []helm.DeleteOption{
		helm.DeletePurge(true),
	}
	_, _ = f.HelmClient.DeleteRelease(ReleaseName, opts...)

	f.TillerTunnel.Close()

	return f.KubeClient.Core().Namespaces().Delete(TestNs, &metav1.DeleteOptions{})
}

func Setup(image, kubeconfig, externalIP string) (*Framework, error) {
	config, err := clientcmd.BuildConfigFromFlags("", kubeconfig)
	if err != nil {
		return nil, err
	}

	kubeclient, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, err
	}

	catalogClientset, err := catalogclientset.NewForConfig(config)
	if err != nil {
		return nil, err
	}

	tunnel, err := portforwarder.New("kube-system", kubeclient, config)
	if err != nil {
		return nil, err
	}
	tillerHost := fmt.Sprintf("127.0.0.1:%d", tunnel.Local)

	cl := helm.NewClient(helm.Host(tillerHost))

	p, err := filepath.Abs(BrokerChartDir)
	if err != nil {
		return nil, err
	}

	opts := []helm.InstallOption{
		helm.ReleaseName(ReleaseName),
		helm.ValueOverrides([]byte(fmt.Sprintf("image: %s\nimagePullPolicy: IfNotPresent", image))),
	}

	_, err = cl.InstallRelease(p, TestNs, opts...)
	if err != nil {
		return nil, err
	}

	f := &Framework{
		CatalogClientset: catalogClientset,
		KubeClient:       kubeclient,
		HelmClient:       cl,
		ExternalIP:       externalIP,
		TillerTunnel:     tunnel,
	}

	if err := f.WaitForClasses(); err != nil {
		return nil, err
	}

	return f, nil
}
