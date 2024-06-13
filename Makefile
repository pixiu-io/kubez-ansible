.PHONY: run build image push clean

tag = v3.0.1
releaseName = kubez-ansible
dockerhubUser = harbor.cloud.pixiuio.com

ALL: run

image:
	docker build --no-cache -t $(dockerhubUser)/pixiuio/$(releaseName):$(tag) -f docker/Dockerfile .

push: image
	docker push $(dockerhubUser)/pixiuio/$(releaseName):$(tag)
