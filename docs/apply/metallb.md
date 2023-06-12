# MetalLB 安装

1. 配置 `/etc/kubez/globals.yml`, 开启metallb选项（默认关闭）
    ``` bash
    enable_metallb: "yes"
    ```
2. 可自定义命名空间（可选）
    ``` bash
    metallb_namespace: "{{ kubez_namespace }}"

    # 可修改为自定义命名空间例如

    metallb_namespace: "metallb-system"
    ```
3. 执行如下命令完成 `metallb` 的安装.
    ``` bash
    # multinode
    kubez-ansible -i multinode apply

    # all-in-one
    kubez-ansible apply
    ```
4. 部署完验证
    ``` bash
    pixiu-system   metallb-controller-64766bb9f9-qq88v         1/1     Running                 0              33m
    pixiu-system   metallb-speaker-tv6z2                       1/1     Running                 0              33m
    ```
5. 创建地址池（官网提供的yaml文件，用来创建可分配的ip地址池）（ 官网地址 https://metallb.universe.tf/configuration/ ）
    ``` bash
    # The address-pools lists the IP addresses that MetalLB is
    # allowed to allocate. You can have as many
    # address pools as you want.
    apiVersion: metallb.io/v1beta1
    kind: IPAddressPool
    metadata:
        # A name for the address pool. Services can request allocation
        # from a specific address pool using this name.
        name: first-pool
        namespace: metallb-system
    spec:
        # A list of IP address ranges over which MetalLB has
        # authority. You can list multiple ranges in a single pool, they
        # will all share the same settings. Each range can be either a
        # CIDR prefix, or an explicit start-end range of IPs.
    addresses:
    - 192.168.10.0/24
    - 192.168.9.1-192.168.9.5
    - fc00:f853:0ccd:e799::/124
    注：
    此文件只做展示，格式不对不可直接复制。
    地址池的命名空间和 matellb 的需保持一致。
    ```
6. 将 servcie 修改为 lb 模式
    ``` bash
    # 查看原有的 service
    pixiu-system   grafana                              ClusterIP      10.254.69.87     <none>        80/TCP                       27s
    # 将 type 字段修改为 LoadBalancer
    sessionAffinity: None
    type: ClusterIP
    # 修改后
    sessionAffinity: None
    type: LoadBalancer
    ```
7. 修改后查看 service
    ``` bash
    pixiu-system   grafana                              LoadBalancer   10.254.69.87     192.168.10.1   80:30325/TCP                 20m
    ```
