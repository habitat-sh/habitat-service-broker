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

package broker

import (
	habv1beta1 "github.com/habitat-sh/habitat-operator/pkg/apis/habitat/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func (b *BrokerLogic) GetHabitat(name, namespace string) (*habv1beta1.Habitat, error) {
	return b.Clients.HabClient.Habitats(namespace).Get(name, metav1.GetOptions{})
}

// CreateHabitat creates a Habitat resource through the Kuberentes client,
// based on the passed Habitat object.
func (b *BrokerLogic) CreateHabitat(habitat *habv1beta1.Habitat, namespace string) error {
	_, err := b.Clients.HabClient.Habitats(namespace).Create(habitat)
	return err
}

func (b *BrokerLogic) UpdateHabitat(habitat *habv1beta1.Habitat, namespace string) error {
	_, err := b.Clients.HabClient.Habitats(namespace).Update(habitat)
	return err
}

// NewHabitat generates a Habitat object based on the passed params.
func NewHabitat(name, image string, params habitatParameters) *habv1beta1.Habitat {
	customVersion := "v1beta2"

	count := 1
	if params.topology == habv1beta1.TopologyLeader {
		// TODO: Atleast 3 instances is a requirement in the
		// habitat-operator for running pods in the leader
		// topology. As a result we hardcode this to 3 for
		// now. We will make this configurable once the
		// restriction is lifted in the operator.
		count = 3
	}

	h := habv1beta1.Habitat{
		TypeMeta: metav1.TypeMeta{
			Kind:       habv1beta1.HabitatKind,
			APIVersion: habv1beta1.SchemeGroupVersion.String(),
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
		Spec: habv1beta1.HabitatSpec{
			V1beta2: &habv1beta1.V1beta2{
				Image: image,
				Count: count,
				Service: habv1beta1.ServiceV1beta2{
					Group:    &params.group,
					Topology: params.topology,
					Name:     name, // This should always be the habitat package name
				},
				PodLabels: params.podLabels,
			},
		},
		CustomVersion: &customVersion,
	}

	if name == "redis" {
		// TODO: The StorageClassName is hardcoded to work with minikube at the
		// moment but should be a passed as an argument to make it work across
		// other providers.
		h.Spec.V1beta2.PersistentStorage = &habv1beta1.PersistentStorage{
			Size:             "128Mi",
			MountPath:        "/hab/svc/redis/data",
			StorageClassName: "standard",
		}
	}

	return &h
}

// DeleteHabitat sends a request to delete a Habitat resource.
func (b *BrokerLogic) DeleteHabitat(habitatName, namespace string) error {
	return b.Clients.HabClient.Habitats(namespace).Delete(habitatName, nil)
}
