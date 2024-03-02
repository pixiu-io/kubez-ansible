.PHONY: run build image push clean

tag = v3.0.1
releaseName = kubez-ansible
dockerhubUser = jacky06

ALL: run

image:
	docker build --no-cache -t $(dockerhubUser)/$(releaseName):$(tag) -f docker/Dockerfile .

push: image
	docker push $(dockerhubUser)/$(releaseName):$(tag)
