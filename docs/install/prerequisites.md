# 依赖安装

- 仅需要在部署节点安装即可，集群运行节点`无需执行`
- `直接安装`或`脚本安装`二选一即可

### 安装步骤

#### 直接安装
   ```shell
   #（推荐）国内快速安装
   curl http://s9.cloud.pixiuio.com/master/setup_env.sh | bash

   #（可选）github 安装，需要网络可用
   curl https://raw.githubusercontent.com/pixiu-io/kubez-ansible/master/tools/setup_env.sh | bash
   ```
#### 脚本安装
   ```text
   # (推荐）国内快速获取脚本
   curl http://s9.cloud.pixiuio.com/master/setup_env.sh -o setup_env.sh

   #（可选）自动获取，网络通时，通过 curl 命令直接获取脚本到本地
   curl https://raw.githubusercontent.com/pixiu-io/kubez-ansible/master/tools/setup_env.sh -o setup_env.sh

   # 手动获取，自动获取失败时使用，一般因为网络不通或者未安装 curl 命令
   # 拷贝项目的 tools/setup_env.sh, 并保存到 setup_env.sh

   # 执行安装脚本
   bash setup_env.sh
   ```

#### 验证
   ```shell
   # 执行 kubez-ansible 有正常回显
   kubez-ansible -h

   # 工作目录（通常是 /root 目录）下自动生成 multinode
   ```
