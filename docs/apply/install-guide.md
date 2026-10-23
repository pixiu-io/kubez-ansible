# Application install guides

1. 配置 `/etc/kubez/globals.yml`, 开启需要开启的 `application` 选项
    ``` bash
    enable_<appication_name>: "yes"
    ```

    已支持的application name：
    - metrics\_server
    - efk
    - helm

2. 执行如下命令完成指定 `applications` 的安装.
    ``` bash
    # multinode
    kubez-ansible -i multinode apply

    # all-in-one
    kubez-ansible apply
    ```

3. 自行验证
