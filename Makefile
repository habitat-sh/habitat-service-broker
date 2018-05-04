# If the USE_SUDO_FOR_DOCKER env var is set, prefix docker commands with 'sudo'
ifdef USE_SUDO_FOR_DOCKER
	SUDO_CMD = sudo
endif

IMAGE ?= kinvolk/habitat-service-broker
TAG ?= $(shell git describe --tags --always)
PULL ?= IfNotPresent

build:
	go build -i github.com/kinvolk/habitat-service-broker/cmd/servicebroker

test:
	go test -v $(shell go list ./... | grep -v /vendor/ | grep -v /test/)

linux:
	GOOS=linux GOARCH=amd64 CGO_ENABLED=0 \
	go build -o servicebroker-linux --ldflags="-s" github.com/kinvolk/habitat-service-broker/cmd/servicebroker

image: linux
	cp servicebroker-linux image/servicebroker
	$(SUDO_CMD) docker build image/ -t "$(IMAGE):$(TAG)"

clean:
	rm -f servicebroker
	rm -f servicebroker-linux

clean-all: deprovision-redis deprovision-nginx
	helm del --purge habitat-service-broker

clean-broker:
	helm del --purge habitat-service-broker
	kubectl delete cm habitat-service-broker -n habitat-service-broker-configuration

push: image
	$(SUDO_CMD) docker push "$(IMAGE):$(TAG)"

deploy-helm: image
	helm install --name habitat-service-broker --namespace habitat-broker \
	charts/habitat-service-broker \
	--set image="$(IMAGE):$(TAG)",imagePullPolicy="$(PULL)"

provision-redis:
	kubectl apply -f manifests/redis/

provision-nginx:
	kubectl apply -f manifests/nginx/

deprovision-redis:
	kubectl delete -f manifests/redis

deprovision-nginx:
	kubectl delete -f manifests/nginx

.PHONY: build test linux image clean clean-all push deploy-helm provision-redis provision-nginx deprovision-redis deprovision-nginx
