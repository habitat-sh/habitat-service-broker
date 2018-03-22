# habitat-service-broker

*Warning: This project is still under development! Should not be used in production.*

Habitat service broker is a broker that implements the [Open Service Broker API](https://github.com/openservicebrokerapi/servicebroker). It is based on the [osb-starter-pack](https://github.com/pmorie/osb-starter-pack).

## Prerequisites

You'll need:

- [`go`](https://golang.org/dl/)
- A running [Kubernetes](https://github.com/kubernetes/kubernetes) cluster
- The [service-catalog](https://github.com/kubernetes-incubator/service-catalog) [installed](https://github.com/kubernetes-incubator/service-catalog/blob/master/docs/install.md) in that cluster
- [Helm](https://helm.sh) [installed](https://docs.helm.sh/using_helm/#quickstart) in the cluster. Make sure [RBAC is correctly configured](https://docs.helm.sh/using_helm/#rbac) for Helm.

## Getting started

Note: Make sure you cloned the repo locally and that all the prerequisites are done before starting.

## Install

Deploy habitat-service-broker using Helm in the running Kubernetes cluster. 

```console
  make deploy-helm
```

## Viewing available classes and plans

The following command shows all the available plans that can be provisioned. Currently there are two plans available to provision, [Redis](https://redis.io/) and [nginx](nginx.com).

```console
  kubectl get clusterserviceclasses -o=custom-columns=NAME:.metadata.name,EXTERNAL\ NAME:.spec.externalName
```

## Provision

The following command deploys an instance of Redis that is running using [Habitat](habitat.sh) in the Kubernetes cluster:

```
  make deploy-redis
```

## Deprovision

To remove the running instance:

```console 
  make deprovision-redis
```
