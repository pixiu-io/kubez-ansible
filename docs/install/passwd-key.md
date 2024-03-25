# multinode 配置文件中写密码/密钥
- 当只有 `ssh_key` 或只有普通用户密码，可以按照如下来配置 `multinode`

    * 配置文件写普通用户的密码
      ```
      [docker-master]
      kube01 ansible_ssh_user=pixiu ansible_ssh_pass=pixiu123. ansible_become=true ansible_become_pass=pixiu123.

      #ansible_ssh_user ssh连接用户
      #ansible_ssh_pass ssh连接用户的密码
      #ansible_become=true 开启sudo命令
      #ansible_become_pass sudo用户的密码，默认是root
      ```
    * 配置文件指定节点的 `ssh_key`
      ```
      [docker-master]
      kube01 ansible_user=root ansible_ssh_private_key_file=/root/root_key

      # ansible_ssh_private_key_file ssh 连接用户的私钥文件
      # key 文件的权限为 600
      ```
