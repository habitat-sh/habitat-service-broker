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

package e2e

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/go-redis/redis"
	utils "github.com/kinvolk/habitat-service-broker/test/e2e/framework"
	catalogv1beta1 "github.com/kubernetes-incubator/service-catalog/pkg/apis/servicecatalog/v1beta1"
	"k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
)

var expectedClusterServiceClasses = []string{
	"1ac7de1d-d89a-41c7-b9a8-744f9256e375", // nginx
	"50e86479-4c66-4236-88fb-a1e61b4c9448", // redis
}

func in(l []catalogv1beta1.ClusterServiceClass, s string) bool {
	for _, v := range l {
		if v.Name == s {
			return true
		}
	}
	return false
}

// TestListClasses checks the broker provides the expected service classes:
// nginx and redis.
func TestListClasses(t *testing.T) {
	classesClient := framework.CatalogClientset.ServicecatalogV1beta1().ClusterServiceClasses()

	list, err := classesClient.List(metav1.ListOptions{})
	if err != nil {
		t.Fatal(err)
	}

	if len(list.Items) != len(expectedClusterServiceClasses) {
		t.Fatalf("expected %d classes, found %d", len(expectedClusterServiceClasses), len(list.Items))
	}

	for _, sc := range expectedClusterServiceClasses {
		if !in(list.Items, sc) {
			t.Fatalf("%q service class not found", sc)
		}
	}
}

func getRedisPassword(items []v1.Secret) (string, error) {
	var redisPassword string

	for _, it := range items {
		if len(it.Data) != 0 {
			secretBytes, ok := it.Data["user.toml"]
			if !ok {
				continue
			}
			passwordParts := strings.Split(string(secretBytes), " = ")
			if len(passwordParts) != 2 {
				return "", fmt.Errorf("error parsing redis password %q", string(secretBytes))
			}
			// secretBytes = `requirepass = "kq3sgkfo0g"`
			quotedPassword := passwordParts[1]
			// quotedPassword = "kq3sgkfo0g"
			redisPassword = strings.Trim(quotedPassword, `"`)
		}
	}

	if redisPassword == "" {
		return "", fmt.Errorf("couldn't find redis password in %+v", items)
	}

	return redisPassword, nil
}

// TestRedisStatefulset creates a service instance of the redis service,
// creates a binding, sets a value in the redis database, unbinds, creates a
// new binding, and checks the value is still present.
//
// This allows us to test that creating services and bindings works, and that
// persistence works.
func TestRedisStatefulset(t *testing.T) {
	siClient := framework.CatalogClientset.ServicecatalogV1beta1().ServiceInstances(utils.TestNs)
	sbClient := framework.CatalogClientset.ServicecatalogV1beta1().ServiceBindings(utils.TestNs)

	siEphemeral, err := utils.ConvertServiceInstances("resources/provision/service-instance.yaml")
	if err != nil {
		t.Fatal(err)
	}

	// 1. create service instance
	_, err = siClient.Create(siEphemeral)
	if err != nil {
		t.Fatal(err)
	}

	if err := framework.WaitForServiceInstanceReady(siEphemeral.Name, utils.TestNs); err != nil {
		t.Fatal(err)
	}

	if err := framework.WaitForPodReady("redis-0", utils.TestNs); err != nil {
		t.Fatal(err)
	}

	// 2. create service binding
	sbEphemeral, err := utils.ConvertServiceBindings("resources/provision/binding.yaml")
	if err != nil {
		t.Fatal(err)
	}

	_, err = sbClient.Create(sbEphemeral)
	if err != nil {
		t.Fatal(err)
	}

	if err := framework.WaitForServiceBindingReady(sbEphemeral.Name, utils.TestNs); err != nil {
		t.Fatal(err)
	}

	if err := framework.WaitForPodReady("redis-0", utils.TestNs); err != nil {
		t.Fatal(err)
	}

	// 3. create k8s service
	svcEphemeral, err := utils.ConvertService("resources/provision/service.yaml")
	if err != nil {
		t.Fatal(err)
	}

	if _, err := framework.KubeClient.Core().Services(utils.TestNs).Create(svcEphemeral); err != nil {
		t.Fatal(err)
	}

	// 4. get credentials from secret
	sl, err := framework.KubeClient.Core().Secrets(utils.TestNs).List(metav1.ListOptions{})
	if err != nil {
		t.Fatal(err)
	}

	redisPassword, err := getRedisPassword(sl.Items)
	if err != nil {
		t.Fatal(err)
	}

	// 5. login to redis
	redisClient := redis.NewClient(&redis.Options{
		Addr:     fmt.Sprintf("%s:%d", framework.ExternalIP, 30001),
		Password: redisPassword,
		DB:       0, // use default DB
	})

	redisKey := "habitat-broker-test"
	expectedValue := "successful"

	// 6. set a value in redis
	if err := wait.Poll(time.Second, time.Minute*1, func() (bool, error) {
		if err := redisClient.Set(redisKey, expectedValue, 0).Err(); err == nil {
			return true, nil
		}

		return false, nil
	}); err != nil {
		t.Fatal(err)
	}

	val, err := redisClient.Get(redisKey).Result()
	if err != nil {
		t.Fatal(err)
	}

	// 7. unbind
	if err := sbClient.Delete(sbEphemeral.Name, &metav1.DeleteOptions{}); err != nil {
		t.Fatal(err)
	}

	if err := framework.WaitForServiceBindingDeleted(sbEphemeral.Name, utils.TestNs); err != nil {
		t.Fatal(err)
	}

	if err := framework.WaitForNoSecrets(utils.TestNs); err != nil {
		t.Fatal(err)
	}

	// 8. create another service binding
	_, err = sbClient.Create(sbEphemeral)
	if err != nil {
		t.Fatal(err)
	}

	if err := framework.WaitForServiceBindingReady(sbEphemeral.Name, utils.TestNs); err != nil {
		t.Fatal(err)
	}

	if err := framework.WaitForPodReady("redis-0", utils.TestNs); err != nil {
		t.Fatal(err)
	}

	// 9. get new credentials
	sl, err = framework.KubeClient.Core().Secrets(utils.TestNs).List(metav1.ListOptions{})
	if err != nil {
		t.Fatal(err)
	}

	redisPassword, err = getRedisPassword(sl.Items)
	if err != nil {
		t.Fatal(err)
	}

	// 10. check the key set in the previous binding still has the right value
	redisClient = redis.NewClient(&redis.Options{
		Addr:     fmt.Sprintf("%s:%d", framework.ExternalIP, 30001),
		Password: redisPassword,
		DB:       0, // use default DB
	})

	if err := wait.Poll(time.Second, time.Minute*1, func() (bool, error) {
		val, err = redisClient.Get(redisKey).Result()
		if err == nil {
			return true, nil
		}

		return false, nil
	}); err != nil {
		t.Fatal(err)
	}

	if val != expectedValue {
		t.Fatalf("wrong value for key %q: expected %q, found %q", redisKey, expectedValue, val)
	}

	// 11. clean up
	if err := sbClient.Delete(sbEphemeral.Name, &metav1.DeleteOptions{}); err != nil {
		t.Fatal(err)
	}

	if err := framework.WaitForServiceBindingDeleted(sbEphemeral.Name, utils.TestNs); err != nil {
		t.Fatal(err)
	}

	if err := siClient.Delete(siEphemeral.Name, &metav1.DeleteOptions{}); err != nil {
		t.Fatal(err)
	}

	if err := framework.WaitForServiceInstanceDeleted(siEphemeral.Name, utils.TestNs); err != nil {
		t.Fatal(err)
	}

	if err := framework.WaitForNoSecrets(utils.TestNs); err != nil {
		t.Fatal(err)
	}
}
