package broker

import (
	habv1beta1 "github.com/habitat-sh/habitat-operator/pkg/apis/habitat/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// CreateHabitat creates a Habitat resource through the Kuberentes client,
// based on the passed Habitat object.
func (b *BrokerLogic) CreateHabitat(habitat *habv1beta1.Habitat) error {
	// TODO: Change namespace in which Habitat is created.
	return b.KubeClient.Client.Post().
		Namespace("default"). // TODO: figure out how to know in which namespace to deploy.
		Resource(habv1beta1.HabitatResourcePlural).
		Body(habitat).
		Do().
		Error()
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
			},
		},
	}
}

// DeleteHabitat sends a request to delete a Habitat resource.
func (b *BrokerLogic) DeleteHabitat(habitatName string) error {
	return b.KubeClient.Client.Delete().
		Namespace("default"). // TODO: figure out how to know in which namespace to deploy.
		Resource(habv1beta1.HabitatResourcePlural).
		Name(habitatName).
		Do().
		Error()
}
