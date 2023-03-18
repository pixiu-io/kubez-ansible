# 离线仓库准备

- 构造离线仓库节点, 用来作为集群节点的仓库源

### 获取 `Nexus` 离线包
1. 自动获取
    ```shell
    # 仓库节点可连网
    curl -fL -u ptx9sk7vk7ow:003a1d6132741b195f332b815e8f98c39ecbcc1a "https://pixiupkg-generic.pkg.coding.net/pixiu/gopixiu-io/nexus.tar.gz?version=v2" -o nexus.tar.gz
    ```
2. 手动获取
    ```shell
    # 不可连网
    拷贝 nexus.tar.gz 到工作目录
    ```

### 安装 Nexus
1. 准备脚本 [setup_registry.sh](https://github.com/gopixiu-io/kubez-ansible/blob/master/tools/setup_registry.sh) 和 `nexus.tar.gz` 处于同一个目录

2. 设置配置文件
    ```shell
    cat > k8senv.yaml << EOF
    # 请填写当前部署机的ip,此处必须修改
    local_ip="localhost"

    # nexus部署的镜像仓库域名,可以不修改
    regis_repos=registry.pixiu.com

    # nexus部署的yum仓库域名, 可以不修改
    mirrors_repos=mirrors.pixiu.com
    EOF
    ```

3. 执行安装
- 确认工作目录中存在 `nexus.tar.gz`, `k8senv.yaml` 和 `setup_nexus.sh` ，并执行
    ```shell
    # 检查
    [root@yum-server ~]# ls
    k8senv.yaml  nexus.tar.gz  setup_nexus.sh

    # 安装
    [root@yum-server ~]# bash setup_nexus.sh
    ```

5. 验证部署
    ```shell
    # 浏览器登陆
    http://<ip>:50000 用户名: admin 密码: admin@AdMin123
    ```
