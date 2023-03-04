# 开启私有仓库

开启私有仓库之后，kubez-ansible将从私有仓库获取构建集群所需镜像

1. 准备私有仓库镜像，在联网的环境中执行 `docker pull jacky06/kube-registry:image_tag` 获得
    ```bash
    image_tag和准备部署的kubernetes release保持一致
    ```

2. 在私有仓库节点自行完成 `docker` 服务的安装

3. 保存获取到的仓库镜像，拷贝并加载到私有仓库节点
    ```bash
    docker save image_id > register.tar

    docker load < register.tar
    ```

4. 拷贝 `tools/setup_registry.sh` script，到私有仓库节点并运行，完成私有仓库的搭建
    ```bash
    a. 默认 registry 的版本: v1.16.2, 服务端口: 4000, 可以根据实际情况修改

    b. 执行前, 确保该节点docker服务处于正常运行状态

    c. 安装完成之后,运行 `curl registry_server_ip:4000/v2/_catalog` 确保registry运行正常
    ```

5. 配置 `/etc/kubez/globals.yml`
    ```bash
    enable_registry: "yes"
    registry_server: `registry_server_ip:4000`
    ```
