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

			// secret contains requirepass and masterauth configurations.
			passwords := strings.Split(string(secretBytes), "\n")
			passwordParts := strings.Split(passwords[0], " = ")
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

func findRedisMasterAndSlave(t *testing.T, services []*v1.Service, password string) (*v1.Service, []*v1.Service, error) {
	var master *v1.Service
	var slaves []*v1.Service

	// t.Logf("\nServices received: %#+v", services)

	for i := range services {
		svc := services[i]
		t.Logf("\n %s: %d\n", svc.Name, svc.Spec.Ports[0].NodePort)
	}

	for i := range services {
		// t.Logf("\nfindRedisMasterAndSlave: hello! %d\n", i)
		svc := services[i]

		port := svc.Spec.Ports[0].NodePort
		t.Logf("\nSelecting port: %d", port)

		redisClient := redis.NewClient(&redis.Options{
			Addr:     fmt.Sprintf("%s:%d", framework.ExternalIP, port),
			Password: password,
			DB:       0, // use default DB
		})

		t.Logf("Waiting for replication info")
		if err := wait.Poll(time.Second, time.Minute*1, func() (bool, error) {
			cmd := redisClient.Info("replication")
			if cmd.Err() == nil {
				t.Logf("\n replication: \n%q\n", cmd.Val())
				// array of "key:value"
				val := strings.Split(cmd.Val(), "\n")

				// first line is the heading of the section, so we ignore that.
				//
				// Expected value is "role:slave" or "role:master"
				t.Logf("\n role: %q\n", val[1])
				role := strings.Split(val[1], ":")
				if strings.Trim(role[1], "\r") == "master" {
					t.Logf("\nSettings master\n")
					master = svc
					t.Logf("\nSet master\n")
				} else {
					t.Logf("\n Adding to slaves\n")
					slaves = append(slaves, svc)
					t.Logf("\n Added to slaves\n")
				}

				return true, nil
			}

			return false, nil
		}); err != nil {
			return nil, nil, err
		}

	}

	t.Logf("\nTime to return\n")
	return master, slaves, nil
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

	// 3. create k8s services
	for i := 0; i <= 2; i++ {
		manifest := fmt.Sprintf("resources/provision/service-%d.yaml", i)

		svcEphemeral, err := utils.ConvertService(manifest)
		if err != nil {
			t.Fatal(err)
		}

		if _, err := framework.KubeClient.Core().Services(utils.TestNs).Create(svcEphemeral); err != nil {
			t.Fatal(err)
		}
	}

	services, err := framework.KubeClient.Core().Services(utils.TestNs).List(metav1.ListOptions{})
	if err != nil {
		t.Fatal(err)
	}

	var redisServices []*v1.Service
	// for _, svc := range services.Items {
	// 	t.Logf("\n %s: %q\n", svc.Name, svc.Spec.Ports)
	// }

	for i := range services.Items {
		svc := services.Items[i]
		if strings.HasPrefix(svc.Name, "redis-service-") {
			redisServices = append(redisServices, &svc)
		}
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

	t.Logf("Redispassword: %#+v", redisPassword)
	// for i, svc := range services.Items {
	// 	t.Logf("%d: %#+v\n", i, svc)
	// }

	// for i := range redisServices {
	// 	svc := redisServices[i]
	// 	t.Logf("\n %s: %d\n", svc.Name, svc.Spec.Ports[0].NodePort)
	// }

	master, slaves, err := findRedisMasterAndSlave(t, redisServices, redisPassword)
	if err != nil {
		t.Fatal(err)
	}

	t.Logf("\n Found master and slave \n")

	t.Logf("\nFound master: %q", master.Name)
	t.Logf("\nFound slaves: %q %q", slaves[0].Name, slaves[1].Name)

	// 5. login to redis
	redisClient := redis.NewClient(&redis.Options{
		Addr:     fmt.Sprintf("%s:%d", framework.ExternalIP, master.Spec.Ports[0].NodePort),
		Password: redisPassword,
		DB:       0, // use default DB
	})

	redisKey := "habitat-broker-test"
	expectedValue := "successful"

	// 6-a. set a value in redis
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

	if val != expectedValue {
		t.Fatalf("wrong value for key %q: expected %q, found %q", redisKey, expectedValue, val)
	}

	// 6-b. retrieve the value from slaves
	for _, svc := range slaves {
		redisClient := redis.NewClient(&redis.Options{
			Addr:     fmt.Sprintf("%s:%d", framework.ExternalIP, svc.Spec.Ports[0].NodePort),
			Password: redisPassword,
			DB:       0, // use default DB
		})

		if err := wait.Poll(time.Second, time.Minute*1, func() (bool, error) {
			val, err := redisClient.Get(redisKey).Result()
			if err == nil {
				if val != expectedValue {
					t.Fatalf("wrong value for key %q: expected %q, found %q", redisKey, expectedValue, val)
				}

				return true, nil
			}

			return false, nil

		}); err != nil {
			t.Fatal(err)
		}
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
	for _, svc := range redisServices {

		redisClient = redis.NewClient(&redis.Options{
			Addr:     fmt.Sprintf("%s:%d", framework.ExternalIP, svc.Spec.Ports[0].NodePort),
			Password: redisPassword,
			DB:       0, // use default DB
		})

		if err := wait.Poll(time.Second, time.Minute*1, func() (bool, error) {
			val, err = redisClient.Get(redisKey).Result()
			if err == nil {
				if val != expectedValue {
					t.Fatalf("wrong value for key %q: expected %q, found %q", redisKey, expectedValue, val)
				}

				return true, nil
			}

			return false, nil
		}); err != nil {
			t.Fatal(err)
		}
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
