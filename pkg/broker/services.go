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

import osb "github.com/pmorie/go-open-service-broker-client/v2"

func boolPtr(b bool) *bool {
	return &b
}

func nginxService() osb.Service {
	return osb.Service{
		Name:          "nginx-habitat",
		ID:            "1ac7de1d-d89a-41c7-b9a8-744f9256e375",
		Description:   "Nginx packaged with Habitat",
		Bindable:      false,
		PlanUpdatable: boolPtr(false),
		Metadata: map[string]interface{}{
			"displayName": "Habitat Nginx service",
			"imageUrl":    "https://avatars2.githubusercontent.com/u/19862012?s=200&v=4",
		},
		Plans: []osb.Plan{
			{
				Name:        "default",
				ID:          "86064792-7ea2-467b-af93-ac9694d96d5b",
				Description: "The default plan for the Nginx Habitat service",
				Free:        boolPtr(true),
				Schemas: &osb.Schemas{
					ServiceInstance: &osb.ServiceInstanceSchema{
						Create: &osb.InputParametersSchema{
							Parameters: map[string]interface{}{
								"$schema": "http://json-schema.org/draft-04/schema",
								"type":    "object",
								"title":   "Parameters",
								"properties": map[string]interface{}{
									"name": map[string]interface{}{
										"title":     "Some Name",
										"type":      "string",
										"maxLength": 63,
										"default":   "My Name",
									},
									"color": map[string]interface{}{
										"title":   "Color",
										"type":    "string",
										"default": "Clear",
										"enum": []string{
											"Clear",
											"Beige",
											"Grey",
										},
									},
								},
							},
						},
					},
				},
			},
		},
	}
}

func redisService() osb.Service {
	return osb.Service{
		Name:          "redis-habitat",
		ID:            "50e86479-4c66-4236-88fb-a1e61b4c9448",
		Description:   "Redis packaged with Habitat",
		Bindable:      true,
		PlanUpdatable: boolPtr(false),
		Metadata: map[string]interface{}{
			"displayName": "Habitat Redis service",
			"imageUrl":    "https://avatars2.githubusercontent.com/u/19862012?s=200&v=4",
		},
		Plans: []osb.Plan{
			{
				Name:        "default",
				ID:          "002341cf-f895-49f4-ba04-bb70291b895c",
				Description: "The default plan for the Redis Habitat service",
				Free:        boolPtr(true),
				Schemas: &osb.Schemas{
					ServiceInstance: &osb.ServiceInstanceSchema{
						Create: &osb.InputParametersSchema{
							Parameters: map[string]interface{}{
								"$schema": "http://json-schema.org/draft-04/schema",
								"type":    "object",
								"title":   "Parameters",
								"properties": map[string]interface{}{
									"name": map[string]interface{}{
										"title":     "Some Name",
										"type":      "string",
										"maxLength": 63,
										"default":   "My Name",
									},
									"color": map[string]interface{}{
										"title":   "Color",
										"type":    "string",
										"default": "Clear",
										"enum": []string{
											"Clear",
											"Beige",
											"Grey",
										},
									},
								},
							},
						},
					},
				},
			},
		},
	}
}
