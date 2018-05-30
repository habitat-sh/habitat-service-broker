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
	"net/http"
	"sync"
	"time"

	"github.com/golang/glog"
	habv1beta1 "github.com/habitat-sh/habitat-operator/pkg/apis/habitat/v1beta1"
	habclient "github.com/habitat-sh/habitat-operator/pkg/client/clientset/versioned/typed/habitat/v1beta1"
	osb "github.com/pmorie/go-open-service-broker-client/v2"
	"github.com/pmorie/osb-broker-lib/pkg/broker"
	v1 "k8s.io/api/core/v1"
	k8sErrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

// NewBrokerLogic is a hook that is called with the Options the program is run with.
func NewBrokerLogic(o *Options, clients *Clients) (*BrokerLogic, error) {
	return &BrokerLogic{
		async:   o.Async,
		Clients: clients,
	}, nil
}

// BrokerLogic provides an implementation of the broker.BrokerLogic interface.
type BrokerLogic struct {
	// Indicates if the broker should handle the requests asynchronously.
	async bool
	// Synchronize go routines.
	sync.RWMutex
	Clients *Clients

	ConfigNamespace *v1.Namespace
	ConfigMap       *v1.ConfigMap
}

// Clients stores all the information specfic to Kubernetes.
type Clients struct {
	KubeClient kubernetes.Interface
	HabClient  habclient.HabitatV1beta1Interface
}

var _ broker.Interface = &BrokerLogic{}

// GetOrCreateNamespace checks if a namespace already exists by the
// given name or else creates one. It sets the namespace object to
// BrokerLogic.ConfigNamespace if successful or else returns the error.
func (b *BrokerLogic) GetOrCreateNamespace(name string) error {
	if ns, err := b.Clients.KubeClient.CoreV1().Namespaces().Get(name, metav1.GetOptions{}); err == nil {
		b.ConfigNamespace = ns
		return nil
	}

	namespace := &v1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
	}

	ns, err := b.Clients.KubeClient.CoreV1().
		Namespaces().
		Create(namespace)
	if err != nil {
		return err
	}

	b.ConfigNamespace = ns
	return nil
}

// GetOrCreateConfigMap checks if a configmap already exists by the
// given name in the given namespace or else creates one. It sets the
// configmap object to BrokerLogic.ConfigMap if successful or else
// returns the error.
func (b *BrokerLogic) GetOrCreateConfigMap(name, namespace string) error {
	if cm, err := b.Clients.KubeClient.CoreV1().ConfigMaps(namespace).Get(name, metav1.GetOptions{}); err == nil {
		b.ConfigMap = cm
		return nil
	}

	configMap := &v1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
	}

	cm, err := b.Clients.KubeClient.CoreV1().
		ConfigMaps(namespace).
		Create(configMap)
	if err != nil {
		return err
	}

	b.ConfigMap = cm
	return nil
}

func (b *BrokerLogic) addToConfigMap(key, value string) error {
	if b.ConfigMap.Data != nil {
		b.ConfigMap.Data[key] = value
	} else {
		b.ConfigMap.Data = map[string]string{
			key: value,
		}
	}

	cm, err := b.Clients.KubeClient.
		CoreV1().
		ConfigMaps(b.ConfigMap.ObjectMeta.Namespace).
		Update(b.ConfigMap)
	if err != nil {
		// Restore ConfigMap.Data to original state
		delete(b.ConfigMap.Data, key)
		return err
	}

	b.ConfigMap = cm
	return nil
}

func (b *BrokerLogic) removeFromConfigMap(key string) error {
	if b.ConfigMap.Data == nil {
		return fmt.Errorf("config map is empty")
	}

	value, ok := b.ConfigMap.Data[key]
	if !ok {
		return fmt.Errorf("key %q not found in the config map", key)
	}

	delete(b.ConfigMap.Data, key)

	cm, err := b.Clients.KubeClient.
		CoreV1().
		ConfigMaps(b.ConfigMap.ObjectMeta.Namespace).
		Update(b.ConfigMap)
	if err != nil {
		// Restore ConfigMap.Data to original state
		b.ConfigMap.Data[key] = value
		return err
	}

	b.ConfigMap = cm
	return nil
}

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

	topology, err := getTopology(request.Parameters)
	if err != nil {
		return nil, err
	}

	group, err := getGroup(request.Parameters)
	if err != nil {
		return nil, err
	}

	params := habitatParameters{
		group:    group,
		topology: topology,
	}

	hab, err := generateHabitatObject(request.PlanID, params)
	if err != nil {
		return nil, err
	}

	ns, err := getNamespace(request.Context)
	if err != nil {
		return nil, err
	}

	err = b.createHabitatResource(hab, ns, request.InstanceID)
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

	err := b.deleteResources(request.PlanID, request.InstanceID)
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
	b.Lock()
	defer b.Unlock()

	response := broker.UnbindResponse{}

	if request.AcceptsIncomplete {
		response.Async = b.async
	}

	err := b.deleteBinding(request)
	if err != nil {
		glog.Warningf("Error in unbind: %q", err)
		return nil, err
	}

	return &response, nil
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

var topologySet = map[habv1beta1.Topology]struct{}{
	habv1beta1.TopologyStandalone: {},
	habv1beta1.TopologyLeader:     {},
}

type habitatParameters struct {
	group    string
	topology habv1beta1.Topology
}

func getTopology(params map[string]interface{}) (habv1beta1.Topology, error) {
	t, ok := params["topology"]
	if !ok {
		return habv1beta1.TopologyStandalone, nil
	}

	s, ok := t.(string)
	if !ok {
		return habv1beta1.Topology(""), fmt.Errorf("topology %q is invalid", t)
	}

	topology := habv1beta1.Topology(s)
	_, ok = topologySet[topology]
	if !ok {
		return habv1beta1.Topology(""), fmt.Errorf("topology %q is invalid", t)
	}

	return topology, nil
}

func getGroup(params map[string]interface{}) (string, error) {
	g, ok := params["group"]
	if !ok {
		return "default", nil
	}

	s, ok := g.(string)
	if !ok {
		return "", fmt.Errorf("group %q is invalid", g)
	}

	return s, nil
}

func getNamespace(context map[string]interface{}) (string, error) {
	namespaceInterface := context["namespace"]
	ns, ok := namespaceInterface.(string)
	if !ok {
		return "", fmt.Errorf(`key "namespace" in context is not a string`)
	}

	return ns, nil
}

func getNamespaceConfigMapKey(name string) string {
	return fmt.Sprintf("%s.namespace", name)
}

func generateHabitatObject(planID string, params habitatParameters) (*habv1beta1.Habitat, error) {
	n, i, err := matchService(planID)
	if err != nil {
		return nil, err
	}

	// Generate Habitat object based on service.
	hab := NewHabitat(n, i, params)

	return hab, nil
}

func (b *BrokerLogic) deleteResources(planID, instanceID string) error {
	name, _, err := matchService(planID)
	if err != nil {
		return err
	}

	key := getNamespaceConfigMapKey(instanceID)
	// TODO: Should we fetch from the API instead?
	ns, ok := b.ConfigMap.Data[key]
	if !ok {
		msg := fmt.Sprintf("could not find namespace for instance %s in configmap %s", instanceID, b.ConfigMap.Name)
		return osb.HTTPStatusCodeError{
			StatusCode:   http.StatusNotFound,
			ErrorMessage: &msg,
		}
	}

	err = b.DeleteHabitat(name, ns)
	if err != nil {
		return err
	}

	return b.removeFromConfigMap(key)
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

func (b *BrokerLogic) createHabitatResource(hab *habv1beta1.Habitat, namespace, instanceID string) error {
	if err := b.CreateHabitat(hab, namespace); err != nil {
		return err
	}

	key := getNamespaceConfigMapKey(instanceID)
	return b.addToConfigMap(key, namespace)
}

func (b *BrokerLogic) createBinding(request *osb.BindRequest) error {
	name, _, err := matchService(request.PlanID)
	if err != nil {
		return err
	}

	key := getNamespaceConfigMapKey(request.InstanceID)
	ns, ok := b.ConfigMap.Data[key]
	if !ok {
		msg := fmt.Sprintf("could not find namespace for instance %s in configmap %s", request.InstanceID, b.ConfigMap.Name)
		return osb.HTTPStatusCodeError{
			StatusCode:   http.StatusNotFound,
			ErrorMessage: &msg,
		}
	}

	switch name {
	case "redis":
		dataString := fmt.Sprintf("requirepass = %q", randSeq(10))
		secret, err := b.createSecret("habitat-osb-redis", "user.toml", dataString, ns)
		if err != nil {
			return err
		}

		err = b.verifySecretExists(secret.Name, ns)
		if err != nil {
			return err
		}

		hab, err := b.GetHabitat(name, ns)
		if err != nil {
			return err
		}

		hab.Kind = habv1beta1.HabitatKind
		hab.APIVersion = habv1beta1.SchemeGroupVersion.String()
		hab.Spec.V1beta2.Service.ConfigSecretName = &secret.Name

		err = b.UpdateHabitat(hab, ns)
		if err != nil {
			return err
		}
	default:
		return fmt.Errorf("Binding for %q is not implemented.", name)
	}

	return nil
}

func (b *BrokerLogic) deleteBinding(request *osb.UnbindRequest) error {
	name, _, err := matchService(request.PlanID)
	if err != nil {
		return fmt.Errorf("error matching service: %v", err)
	}

	key := getNamespaceConfigMapKey(request.InstanceID)
	// TODO: Should we fetch the configmap from the API instead?
	ns, ok := b.ConfigMap.Data[key]
	if !ok {
		msg := fmt.Sprintf("could not find namespace for instance %s in configmap %s", request.InstanceID, b.ConfigMap.Name)
		return osb.HTTPStatusCodeError{
			StatusCode:   http.StatusNotFound,
			ErrorMessage: &msg,
		}
	}

	hab, err := b.GetHabitat(name, ns)
	if err != nil {
		return fmt.Errorf("error getting Habitat service: %v", err)
	}

	switch name {
	case "redis":
		secretName := hab.Spec.V1beta2.Service.ConfigSecretName
		if secretName == nil {
			return fmt.Errorf("unbinding failed for %q as %q is nil", name, "configSecretName")
		}

		hab.Kind = habv1beta1.HabitatKind
		hab.APIVersion = habv1beta1.SchemeGroupVersion.String()
		hab.Spec.V1beta2.Service.ConfigSecretName = nil

		if err := b.UpdateHabitat(hab, ns); err != nil {
			return fmt.Errorf("error updating habitat: %v", err)
		}

		err := b.deleteSecret(*secretName, ns)
		if err != nil {
			return fmt.Errorf("error deleting secret: %v", err)
		}
	default:
		return fmt.Errorf("unbinding for %q is not implemented.", name)
	}

	return nil
}

func (b *BrokerLogic) createSecret(secretPrefix, dataKey, dataString, namespace string) (*v1.Secret, error) {
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
			return nil, errors.New("max retries exceeded: secret not created")
		default:
		}

		secretName := fmt.Sprintf("%s-%s", secretPrefix, randSeq(5))
		s.ObjectMeta = metav1.ObjectMeta{Name: secretName}
		s.Data = map[string][]byte{dataKey: []byte(dataString)}

		secret, err := b.Clients.KubeClient.CoreV1().Secrets(namespace).Create(s)
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

func (b *BrokerLogic) verifySecretExists(name, namespace string) error {
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
			return errors.New("max retries exceeded: secret not found")
		default:
		}

		_, err := b.Clients.KubeClient.
			CoreV1().
			Secrets(namespace).
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

func (b *BrokerLogic) deleteSecret(name, namespace string) error {
	options := &metav1.DeleteOptions{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Secret",
			APIVersion: "v1",
		},
	}

	// TODO: figure out how to know in which namespace to deploy.
	return b.Clients.KubeClient.
		CoreV1().
		Secrets(namespace).
		Delete(name, options)
}
