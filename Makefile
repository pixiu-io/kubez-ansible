.PHONY: run build image push clean

tag = v3.0.4
releaseName = kubez-ansible
dockerhubUser = crpi-0ecikjs9ylb2hqyo.cn-hangzhou.personal.cr.aliyuncs.com
imageRepo = $(dockerhubUser)/pixiu-public/$(releaseName)
imageName = $(imageRepo):$(tag)
imageNameArm64 = $(imageRepo):$(tag)-arm64

ALL: run

# 本地单架构构建（当前机器，默认 amd64 tag）
image:
	docker build --no-cache -t $(imageName) -f docker/Dockerfile .

# 分别推送 amd64（原 tag）与 arm64（tag-arm64）
push:
	docker buildx build --platform linux/amd64 --push \
		-t $(imageName) -f docker/Dockerfile .
	docker buildx build --platform linux/arm64 --push \
		-t $(imageNameArm64) -f docker/Dockerfile .
