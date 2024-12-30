.PHONY: run build image push clean

tag = v3.0.1
releaseName = kubez-ansible
dockerhubUser = crpi-0ecikjs9ylb2hqyo.cn-hangzhou.personal.cr.aliyuncs.com

ALL: run

image:
	docker build --no-cache -t $(dockerhubUser)/pixiu-public/$(releaseName):$(tag) -f docker/Dockerfile .

push: image
	docker push $(dockerhubUser)/pixiu-public/$(releaseName):$(tag)
