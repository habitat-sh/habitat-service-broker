package broker

import (
	"fmt"
	"sync"

	osb "github.com/pmorie/go-open-service-broker-client/v2"
	"github.com/pmorie/osb-broker-lib/pkg/broker"
	"gopkg.in/yaml.v2"
	"k8s.io/helm/pkg/helm"
)

//TODO (lilic): Add option for tiller to be modified by the user via the cli args.
const (
	tiller = "tiller-deploy.kube-system.svc.cluster.local:44134"
)

// NewBrokerLogic is a hook that is called with the Options the program is run with.
func NewBrokerLogic(o *Options) (*BrokerLogic, error) {
	return &BrokerLogic{
		async: o.Async,
	}, nil
}

// BrokerLogic provides an implementation of the broker.BrokerLogic interface.
type BrokerLogic struct {
	// Indicates if the broker should handle the requests asynchronously.
	async bool
	// Synchronize go routines.
	sync.RWMutex
}

var _ broker.Interface = &BrokerLogic{}

func (b *BrokerLogic) GetCatalog(c *broker.RequestContext) (*osb.CatalogResponse, error) {
	response := &osb.CatalogResponse{}

	// TODO (lilic): At some point move these at a more appropriate place.
	data := `
---
services:
- name: nginx-habitat
  id: 1ac7de1d-d89a-41c7-b9a8-744f9256e375
  description: Nginx packaged with Habitat
  bindable: false
  plan_updateable: false
  metadata:
    displayName: "Habitat Nginx service"
    imageUrl: https://avatars2.githubusercontent.com/u/19862012?s=200&v=4
  plans:
  - name: default
    id: 86064792-7ea2-467b-af93-ac9694d96d5b
    description: The default plan for the Nginx Habitat service
    free: true
    schemas:
      service_instance:
        create:
          "$schema": "http://json-schema.org/draft-04/schema"
          "type": "object"
          "title": "Parameters"
          "properties":
          - "name":
              "title": "Some Name"
              "type": "string"
              "maxLength": 63
              "default": "My Name"
          - "color":
              "title": "Color"
              "type": "string"
              "default": "Clear"
              "enum":
              - "Clear"
              - "Beige"
              - "Grey"
- name: redis-habitat
  id: 50e86479-4c66-4236-88fb-a1e61b4c9448 
  description: Redis packaged with Habitat
  bindable: false
  plan_updateable: false
  metadata:
    displayName: "Habitat Redis service"
    imageUrl: https://avatars2.githubusercontent.com/u/19862012?s=200&v=4
  plans:
  - name: default
    id: 002341cf-f895-49f4-ba04-bb70291b895c
    description: The default plan for the Redis Habitat example service
    free: true
    schemas:
      service_instance:
        create:
          "$schema": "http://json-schema.org/draft-04/schema"
          "type": "object"
          "title": "Parameters"
          "properties":
          - "name":
              "title": "Some Name"
              "type": "string"
              "maxLength": 63
              "default": "My Name"
          - "color":
              "title": "Color"
              "type": "string"
              "default": "Clear"
              "enum":
              - "Clear"
              - "Beige"
              - "Grey"
`

	err := yaml.Unmarshal([]byte(data), &response)
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

	fmt.Println("Provisioning request...")

	err := installHelmChart(request.PlanID)
	if err != nil {
		fmt.Println(err)
		return nil, err
	}

	return &response, nil
}

func (b *BrokerLogic) Deprovision(request *osb.DeprovisionRequest, c *broker.RequestContext) (*osb.DeprovisionResponse, error) {
	b.Lock()
	defer b.Unlock()

	fmt.Println("Started deprovison...")

	response := osb.DeprovisionResponse{}

	if request.AcceptsIncomplete {
		response.Async = b.async
	}

	// Delete Helm release of requested service.
	err := deleteHelm(request.PlanID)
	if err != nil {
		fmt.Println(err)
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

func installHelmChart(id string) error {
	chartPath, chartName, err := matchHelmRelease(id)
	if err != nil {
		return err
	}

	// TODO (lilic): Pass these values from the user options.
	vals, err := yaml.Marshal(map[string]interface{}{})
	if err != nil {
		return err
	}

	helmClient := helm.NewClient(helm.Host(tiller))

	_, err = helmClient.InstallRelease(chartPath, "default", helm.ReleaseName(chartName), helm.ValueOverrides(vals))
	if err != nil {
		// TODO (lilic): Check if Helm installed it correctly and return true errors.
		// We ignore the error and don't return as Helm installs it correctly, despite the Helm error...
	}
	fmt.Printf("Finished installing Helm release for %s", chartName)

	return nil
}

func matchHelmRelease(planID string) (string, string, error) {
	chartPath := ""
	chartName := ""

	if planID == "" {
		return chartPath, chartName, fmt.Errorf("PlanID could not be matched. PlanID was empty.")
	}

	if planID == "002341cf-f895-49f4-ba04-bb70291b895c" {
		chartName = "redis"
		chartPath = "/opt/servicebroker/items/redis-1.1.13.tgz"
	}
	if planID == "86064792-7ea2-467b-af93-ac9694d96d5b" {
		chartName = "nginx"
		chartPath = "/opt/servicebroker/items/nginx-0.1.0.tgz"
	}

	if chartPath == "" || chartName == "" {
		return chartPath, chartName, fmt.Errorf("PlanID could not be matched. No known service for PlanID: %s", planID)
	}

	return chartPath, chartName, nil
}

func deleteHelm(id string) error {
	_, chartName, err := matchHelmRelease(id)
	if err != nil {
		fmt.Println(err)
		return err
	}

	helmClient := helm.NewClient(helm.Host(tiller))
	ops := []helm.DeleteOption{
		helm.DeletePurge(true),
	}

	if _, err := helmClient.DeleteRelease(chartName, ops...); err != nil {
		return err
	}
	return nil
}
