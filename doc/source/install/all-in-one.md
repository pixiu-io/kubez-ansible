# 单节点部署 -- All-in-one

1. 部署前准备--[前提条件](prerequisites.md)

2. all-in-one 环境的自定义安装

    a. 执行如下命令，进行kubernetes的依赖安装

    ``` bash
    kubez-ansible bootstrap-servers
    ```

    b.（可选）使用离线安装模式 -- [开启本地私有仓库](setup-registry.md)

    c. 执行如下命令，进行 `kubernetes` 的依赖安装

    ``` bash
    kubez-ansible deploy
    ```

3. 生成 kubernetes RC 文件 `.kube/config`
   ``` bash
   kubez-ansible post-deploy
   ```

4. 验证环境
   ```bash
   # kubectl get node
   NAME    STATUS   ROLES    AGE    VERSION
   kubez   Ready    master   134d   v1.23.6
   ```
