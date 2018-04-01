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
	"fmt"
	"io/ioutil"
	"sync"

	habv1beta1 "github.com/habitat-sh/habitat-operator/pkg/apis/habitat/v1beta1"
	osb "github.com/pmorie/go-open-service-broker-client/v2"
	"github.com/pmorie/osb-broker-lib/pkg/broker"
	"gopkg.in/yaml.v2"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

// NewBrokerLogic is a hook that is called with the Options the program is run with.
func NewBrokerLogic(o *Options, client *Client) (*BrokerLogic, error) {
	return &BrokerLogic{
		async:      o.Async,
		KubeClient: client,
	}, nil
}

// BrokerLogic provides an implementation of the broker.BrokerLogic interface.
type BrokerLogic struct {
	// Indicates if the broker should handle the requests asynchronously.
	async bool
	// Synchronize go routines.
	sync.RWMutex
	KubeClient *Client
}

// Client stores all the information specfic to Kubernetes.
type Client struct {
	KubeClient kubernetes.Interface
	Client     *rest.RESTClient
}

var _ broker.Interface = &BrokerLogic{}

func (b *BrokerLogic) GetCatalog(c *broker.RequestContext) (*osb.CatalogResponse, error) {
	response := &osb.CatalogResponse{}

	// TODO (lilic): At some point move these at a more appropriate place.
	data, err := ioutil.ReadFile("services.yml")
	if err != nil {
		return nil, err
	}

	err := yaml.Unmarshal(data, &response)
	if err != nil {
		return nil, err
	}

	return response, nil
}

func (b *BrokerLogic) Provision(request *osb.ProvisionRequest, c *broker.RequestContext) (*osb.ProvisionResponse, error) {
	b.Lock()
	defer b.Unlock()

	response := osb.ProvisionResponse{}

	if request.AcceptsIncomplete {
		response.Async = b.async
	}

	hab, err := generateHabitatObject(request.PlanID)
	if err != nil {
		return nil, err
	}

	err = b.createHabitatResource(hab)
	if err != nil {
		return nil, err
	}

	return &response, nil
}

func (b *BrokerLogic) Deprovision(request *osb.DeprovisionRequest, c *broker.RequestContext) (*osb.DeprovisionResponse, error) {
	b.Lock()
	defer b.Unlock()

	response := osb.DeprovisionResponse{}

	if request.AcceptsIncomplete {
		response.Async = b.async
	}

	err := b.deleteResources(request.PlanID)
	if err != nil {
		return nil, err
	}

	return &response, nil
}

func (b *BrokerLogic) LastOperation(request *osb.LastOperationRequest, c *broker.RequestContext) (*osb.LastOperationResponse, error) {
	return nil, nil
}

func (b *BrokerLogic) Bind(request *osb.BindRequest, c *broker.RequestContext) (*osb.BindResponse, error) {
	b.Lock()
	defer b.Unlock()

	response := osb.BindResponse{}

	if request.AcceptsIncomplete {
		response.Async = b.async
	}

	return &response, nil
}

func (b *BrokerLogic) Unbind(request *osb.UnbindRequest, c *broker.RequestContext) (*osb.UnbindResponse, error) {
	return &osb.UnbindResponse{}, nil
}

func (b *BrokerLogic) Update(request *osb.UpdateInstanceRequest, c *broker.RequestContext) (*osb.UpdateInstanceResponse, error) {
	response := osb.UpdateInstanceResponse{}
	if request.AcceptsIncomplete {
		response.Async = b.async
	}

	return &response, nil
}

func (b *BrokerLogic) ValidateBrokerAPIVersion(version string) error {
	return nil
}

func generateHabitatObject(planID string) (*habv1beta1.Habitat, error) {
	n, i, err := matchService(planID)
	if err != nil {
		return nil, err
	}
	// Generate Habitat object based on service.
	hab := NewHabitat(n, i, 1) // TODO: Decide how many instances we should be running?

	return hab, nil
}

func (b *BrokerLogic) deleteResources(planID string) error {
	n, _, err := matchService(planID)
	if err != nil {
		return err
	}

	err = b.DeleteHabitat(n)
	if err != nil {
		return err
	}

	return nil
}

func matchService(planID string) (string, string, error) {
	name := ""
	image := ""

	switch planID {
	case "002341cf-f895-49f4-ba04-bb70291b895c":
		name = "redis"
		image = "kinvolk/osb-redis:latest" // TODO: find a better way than latest!
	case "86064792-7ea2-467b-af93-ac9694d96d5b":
		name = "nginx"
		image = "kinvolk/osb-nginx:latest" // TODO: find a better way than latest!
	case "":
		return name, image, fmt.Errorf("PlanID could not be matched. PlanID was empty.")
	default:
		return name, image, fmt.Errorf("PlanID could not be matched. PlanID did not match existing PlanID.")
	}

	return name, image, nil
}

func (b *BrokerLogic) createHabitatResource(hab *habv1beta1.Habitat) error {
	if err := b.CreateHabitat(hab); err != nil {
		return err
	}

	return nil
}
