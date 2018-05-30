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
	"os"
	"path/filepath"
	"strings"
	"time"

	catalogv1beta1 "github.com/kubernetes-incubator/service-catalog/pkg/apis/servicecatalog/v1beta1"
	"k8s.io/api/core/v1"
	k8sErrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/apimachinery/pkg/util/yaml"
)

// WaitForClasses waits until there's any Cluster Service Class.
func (f *Framework) WaitForClasses() error {
	classesClient := f.CatalogClientset.ServicecatalogV1beta1().ClusterServiceClasses()

	return wait.Poll(time.Second, time.Minute*5, func() (bool, error) {
		cs, err := classesClient.List(metav1.ListOptions{})
		if err != nil {
			return false, err
		}

		if len(cs.Items) == 0 {
			return false, nil
		}

		return true, nil
	})
}

// WaitForServiceInstanceReady waits until the Service Instance is in a Ready state.
func (f *Framework) WaitForServiceInstanceReady(name, namespace string) error {
	siClient := f.CatalogClientset.ServicecatalogV1beta1().ServiceInstances(namespace)

	return wait.Poll(time.Second, time.Minute*1, func() (bool, error) {
		si, err := siClient.Get(name, metav1.GetOptions{})
		if err != nil {
			return false, nil
		}

		for _, c := range si.Status.Conditions {
			if c.Type == catalogv1beta1.ServiceInstanceConditionReady {
				if c.Status == catalogv1beta1.ConditionTrue {
					return true, nil
				}
			}
		}

		return false, nil
	})
}

// WaitForServiceBindingReady waits until the Service Binding is in a Ready state.
func (f *Framework) WaitForServiceBindingReady(name, namespace string) error {
	sbClient := f.CatalogClientset.ServicecatalogV1beta1().ServiceBindings(namespace)

	return wait.Poll(time.Second, time.Minute*1, func() (bool, error) {
		si, err := sbClient.Get(name, metav1.GetOptions{})
		if err != nil {
			return false, nil
		}

		for _, c := range si.Status.Conditions {
			if c.Type == catalogv1beta1.ServiceBindingConditionReady {
				if c.Status == catalogv1beta1.ConditionTrue {
					return true, nil
				}
			}
		}

		return false, nil
	})
}

// WaitForServiceInstanceDeleted waits until the Service Instance is deleted.
func (f *Framework) WaitForServiceInstanceDeleted(name, namespace string) error {
	siClient := f.CatalogClientset.ServicecatalogV1beta1().ServiceInstances(namespace)

	return wait.Poll(time.Second, time.Minute*1, func() (bool, error) {
		_, err := siClient.Get(name, metav1.GetOptions{})
		if err != nil && k8sErrors.IsNotFound(err) {
			return true, nil
		}

		return false, nil
	})

}

// WaitForServiceBindingDeleted waits until the Service Binding is deleted.
func (f *Framework) WaitForServiceBindingDeleted(name, namespace string) error {
	siClient := f.CatalogClientset.ServicecatalogV1beta1().ServiceBindings(namespace)

	return wait.Poll(time.Second, time.Minute*1, func() (bool, error) {
		_, err := siClient.Get(name, metav1.GetOptions{})
		if err != nil && k8sErrors.IsNotFound(err) {
			return true, nil
		}

		return false, nil
	})

}

// WaitForNoSecrets waits until there is no secrets except the default token
// and the habitat-service-broker token.
func (f *Framework) WaitForNoSecrets(namespace string) error {
	return wait.Poll(time.Second, time.Minute*1, func() (bool, error) {
		sl, err := f.KubeClient.Core().Secrets(namespace).List(metav1.ListOptions{})
		if err != nil {
			return false, err
		}

		if len(sl.Items) != 2 {
			return false, nil
		}

		for _, s := range sl.Items {
			if !(strings.HasPrefix(s.Name, "default-token-") ||
				strings.HasPrefix(s.Name, "habitat-service-broker-")) {
				return false, fmt.Errorf("unexpected secret %q", s.Name)
			}
		}

		return true, nil
	})
}

// WaitForPodReady waits until the pod is in a Ready state.
func (f *Framework) WaitForPodReady(name, namespace string) error {
	return wait.Poll(time.Second, time.Minute*1, func() (bool, error) {
		p, err := f.KubeClient.Core().Pods(namespace).Get(name, metav1.GetOptions{})
		if err != nil {
			return false, nil
		}

		for _, c := range p.Status.Conditions {
			if c.Type == v1.PodReady {
				if c.Status == v1.ConditionTrue {
					return true, nil
				}
			}
		}

		return false, nil
	})
}

// ConvertServiceInstances takes in a path to the YAML file containing the manifest.
// It converts the file to the Service Instances object.
func ConvertServiceInstances(pathToYaml string) (*catalogv1beta1.ServiceInstance, error) {
	si := catalogv1beta1.ServiceInstance{}

	if err := convertToK8sResource(pathToYaml, &si); err != nil {
		return nil, err
	}

	return &si, nil
}

// ConvertServiceBindings takes in a path to the YAML file containing the manifest.
// It converts the file to the Service Binding object.
func ConvertServiceBindings(pathToYaml string) (*catalogv1beta1.ServiceBinding, error) {
	si := catalogv1beta1.ServiceBinding{}

	if err := convertToK8sResource(pathToYaml, &si); err != nil {
		return nil, err
	}

	return &si, nil
}

// ConvertService takes in a path to the YAML file containing the manifest.
// It converts the file to the Service object.
func ConvertService(pathToYaml string) (*v1.Service, error) {
	s := v1.Service{}

	if err := convertToK8sResource(pathToYaml, &s); err != nil {
		return nil, err
	}

	return &s, nil
}

// pathToOSFile takes in a path and converts it to a File.
func pathToOSFile(relativePath string) (*os.File, error) {
	path, err := filepath.Abs(relativePath)
	if err != nil {
		return nil, err
	}

	manifest, err := os.Open(path)
	if err != nil {
		return nil, err
	}

	return manifest, nil
}

func convertToK8sResource(pathToYaml string, into runtime.Object) error {
	manifest, err := pathToOSFile(pathToYaml)
	if err != nil {
		return err
	}

	if err := yaml.NewYAMLToJSONDecoder(manifest).Decode(into); err != nil {
		return err
	}

	return nil
}
