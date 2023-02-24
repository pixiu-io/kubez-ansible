# 批量开启免密码登陆

### 依赖条件
- [依赖安装](prerequisites.md)
- 配置部署节点的 `/etc/hosts` , 添加 `kubernetes` 节点的ip和主机名解析
- 完成 `multinode` 配置

### 执行步骤
1. 执行如下命令，进行开启批量免密码登陆
    ``` bash
    kubez-ansible -i multinode  authorized-key
    ```

2. 在出现 `SSH password:` 的提示后，输入节点密码，完成批量开通

3. 登陆验证
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
