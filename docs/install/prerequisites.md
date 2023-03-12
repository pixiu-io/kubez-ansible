# 依赖安装

- 仅需要在部署节点安装即可，集群运行节点`无需执行`

### 安装
1. 获取安装脚本
   ```shell
   # 自动获取，当有网络的时候建议直接安装(因为简单又方便)
   curl https://raw.githubusercontent.com/gopixiu-io/kubez-ansible/master/tools/setup_env.sh -o setup_env.sh

   # 手动获取，自动获取失败时使用，一般因为网络不通或者未安装 curl 命令
   # 拷贝项目的 tools/setup_env.sh，并保存到 setup_env.sh
   ```

2. 执行安装脚本
   ``` shell
   bash setup_env.sh
   ```

3. 验证
   ```shell
   # 执行 kubez-ansible 有正常回显
   kubez-ansible -h

   # 工作目录（通常是 /root 目录）下自动生成 multinode
   ```
