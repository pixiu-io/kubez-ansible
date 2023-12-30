# 批量开启免密码登陆

### 依赖条件
- [依赖安装](prerequisites.md)
- 配置部署节点的 `/etc/hosts` , 添加 `kubernetes` 节点的ip和主机名解析
- 完成 `multinode` 配置
- 依赖 `sshpass` 包

### 执行步骤
1. （可选）生成 `id_rsa` 和 `id_rsa.pub` 文件
    ```bash
    # 进入 /root/.ssh 文件夹， 查看是否有 id_rsa 和 id_rsa.pub 文件。如果有，则忽略此步骤；如果没有，请执行下面的命令生成
    ssh-keygen
    ```

2. 确保 `sshpass` 已安装
    ``` bash
    # CentOS
    # yum -y install sshpass
    # Ubuntu/Debian
    # apt-get install sshpass
    ```

3. 执行如下命令，进行开启批量免密码登陆
    ``` bash
    kubez-ansible -i multinode  authorized-key
    ```

4. 在出现 `SSH password:` 的提示后，输入节点密码，完成批量开通

5. 登陆验证
    ``` bash
    ansible -i multinode all -m ping

    # 成功回显
    pixiu | SUCCESS => {
    "ansible_facts": {
    "discovered_interpreter_python": "/usr/bin/python"
    },
    "changed": false,
    "ping": "pong"
    }
    ```
