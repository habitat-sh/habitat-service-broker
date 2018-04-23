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

func (b *BrokerLogic) GetHabitat(name string) (*habv1beta1.Habitat, error) {
	return b.Clients.HabClient.Get(name, metav1.GetOptions{})
}

// CreateHabitat creates a Habitat resource through the Kuberentes client,
// based on the passed Habitat object.
func (b *BrokerLogic) CreateHabitat(habitat *habv1beta1.Habitat) error {
	_, err := b.Clients.HabClient.Create(habitat)
	return err
}

func (b *BrokerLogic) UpdateHabitat(habitat *habv1beta1.Habitat) error {
	_, err := b.Clients.HabClient.Update(habitat)
	return err
}

// NewHabitat generates a Habitat object based on the passed params.
func NewHabitat(name, image string, count int) *habv1beta1.Habitat {
	return &habv1beta1.Habitat{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Habitat",            //TODO: take from hab-operator
			APIVersion: "habitat.sh/v1beta1", //TODO: take from hab-operator
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
		Spec: habv1beta1.HabitatSpec{
			Image: image,
			Count: count,
			Service: habv1beta1.Service{
				Group:    "default",
				Topology: habv1beta1.TopologyStandalone,
				Name:     name, // This should always be the habitat package name
			},
		},
	}
}

// DeleteHabitat sends a request to delete a Habitat resource.
func (b *BrokerLogic) DeleteHabitat(habitatName string) error {
	return b.Clients.HabClient.Delete(habitatName, nil)
}
