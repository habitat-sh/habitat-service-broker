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
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/golang/glog"
	habv1beta1 "github.com/habitat-sh/habitat-operator/pkg/apis/habitat/v1beta1"
	osb "github.com/pmorie/go-open-service-broker-client/v2"
	"github.com/pmorie/osb-broker-lib/pkg/broker"
	v1 "k8s.io/api/core/v1"
	k8sErrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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

func (b *BrokerLogic) GetCatalog(c *broker.RequestContext) (*broker.CatalogResponse, error) {
	response := &broker.CatalogResponse{
		CatalogResponse: osb.CatalogResponse{
			Services: []osb.Service{
				nginxService(),
				redisService(),
			},
		},
	}

	return response, nil
}

func (b *BrokerLogic) Provision(request *osb.ProvisionRequest, c *broker.RequestContext) (*broker.ProvisionResponse, error) {
	b.Lock()
	defer b.Unlock()

	response := broker.ProvisionResponse{}

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

func (b *BrokerLogic) Deprovision(request *osb.DeprovisionRequest, c *broker.RequestContext) (*broker.DeprovisionResponse, error) {
	b.Lock()
	defer b.Unlock()

	response := broker.DeprovisionResponse{}

	if request.AcceptsIncomplete {
		response.Async = b.async
	}

	err := b.deleteResources(request.PlanID)
	if err != nil {
		return nil, err
	}

	return &response, nil
}

func (b *BrokerLogic) LastOperation(request *osb.LastOperationRequest, c *broker.RequestContext) (*broker.LastOperationResponse, error) {
	return nil, nil
}

func (b *BrokerLogic) Bind(request *osb.BindRequest, c *broker.RequestContext) (*broker.BindResponse, error) {
	b.Lock()
	defer b.Unlock()

	response := broker.BindResponse{}

	if request.AcceptsIncomplete {
		response.Async = b.async
	}

	err := b.createBinding(request)
	if err != nil {
		return nil, err
	}

	response.Exists = true
	return &response, nil
}

func (b *BrokerLogic) Unbind(request *osb.UnbindRequest, c *broker.RequestContext) (*broker.UnbindResponse, error) {
	return &broker.UnbindResponse{}, nil
}

func (b *BrokerLogic) Update(request *osb.UpdateInstanceRequest, c *broker.RequestContext) (*broker.UpdateInstanceResponse, error) {
	response := broker.UpdateInstanceResponse{}
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

func (b *BrokerLogic) createBinding(request *osb.BindRequest) error {
	name, _, err := matchService(request.PlanID)
	if err != nil {
		return err
	}

	switch name {
	case "redis":
		dataString := fmt.Sprintf("requirepass = %q", randSeq(10))
		secret, err := b.createSecret("habitat-osb-redis", "user.toml", dataString)
		if err != nil {
			return err
		}

		err = b.verifySecretExists(secret.Name)
		if err != nil {
			return err
		}

		hab, err := b.GetHabitat(name)
		if err != nil {
			return err
		}

		hab.Kind = "Habitat"
		hab.APIVersion = "habitat.sh/v1beta1"
		hab.Spec.Service.ConfigSecretName = secret.Name

		err = b.UpdateHabitat(hab)
		if err != nil {
			return err
		}
	default:
		return fmt.Errorf("Binding for %q is not implemented.", name)
	}

	return nil
}

func (b *BrokerLogic) createSecret(secretPrefix, dataKey, dataString string) (*v1.Secret, error) {
	s := &v1.Secret{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Secret",
			APIVersion: "v1",
		},
		Type: v1.SecretTypeOpaque,
	}

	timer := time.NewTimer(15 * time.Second)
	defer timer.Stop()

	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-timer.C:
			return nil, errors.New("Max retries exceeded: secret not created")
		default:
		}

		secretName := fmt.Sprintf("%s-%s", secretPrefix, randSeq(5))
		s.ObjectMeta = metav1.ObjectMeta{Name: secretName}
		s.Data = map[string][]byte{dataKey: []byte(dataString)}

		// TODO: figure out how to know in which namespace to deploy.
		secret, err := b.KubeClient.KubeClient.CoreV1().Secrets("default").Create(s)
		if err == nil {
			return secret, nil
		}

		switch e := err.(type) {
		case *k8sErrors.StatusError:
			// We will only retry if there's a clash in the secret's name
			// or else let the caller of this method handle the error.
			if e.ErrStatus.Reason != metav1.StatusReasonAlreadyExists {
				return nil, err
			}
		default:
			// We will not handle any other kind of error.
			return nil, err
		}

		glog.Warningf("secret with name %s already exists. Trying again with a different name...", secretName)

		<-ticker.C
	}
}

func (b *BrokerLogic) verifySecretExists(name string) error {
	options := metav1.GetOptions{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Secret",
			APIVersion: "v1",
		},
		IncludeUninitialized: false,
	}

	timer := time.NewTimer(15 * time.Second)
	defer timer.Stop()

	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	for {

		select {
		case <-timer.C:
			return errors.New("Max retries exceeded: secret not found")
		default:
		}

		// TODO: figure out how to know in which namespace to deploy.
		_, err := b.KubeClient.KubeClient.
			CoreV1().
			Secrets("default").
			Get(name, options)
		if err == nil {
			return nil
		}

		switch e := err.(type) {
		case *k8sErrors.StatusError:
			// We will only retry if the secret was not found (probably
			// because it wasn't created yet) or else let the caller
			// of this method handle the error.
			if e.ErrStatus.Reason != metav1.StatusReasonNotFound {
				return err
			}
		default:
			// We will not handle any other kind of error.
			return err
		}

		glog.Warningf("secret with name %s not found yet, trying again...", name)

		<-ticker.C
	}
}
