## kubez-ansible 集成程序说明

### 1：集成说明（以为 Jenkins 为例子）

#### 1.1：在 https://artifacthub.io 这个⽹址找到 Jenkins
![image](https://github.com/gitlayzer/images/assets/77761224/d7f45a43-1c25-4132-b4d7-96a9441beb1a)
#### 1.2：我们查看它的 Values 文件
![image](https://github.com/gitlayzer/images/assets/77761224/02821dc0-1788-4660-90c2-80aded599540)
![image](https://github.com/gitlayzer/images/assets/77761224/b7a270aa-bd86-40c2-a350-4221ab0b40ce)

#### 在这⾥我们抽出来我们需要的键值对，这⾥你可以先⼿动使⽤ helm 安装之后看下都有哪些默认的可以修改的键值对
```yaml
jenkins:
  name: jenkins
  namespace: "{{ jenkins_namespace }}"
  repository:
    name: jenkinsci
    url: https://charts.jenkins.io/
  chart:
    path: jenkinsci/jenkins
    version: 4.2.20
  chart_extra_vars:
    persistence.storageClass: "{{ jenkins_storage_class }}"
    persistence.size: "{{ jenkins_storage_size }}"
    controller.adminPassword: "{{ initial_admin_password }}"
```

#### 这⾥的配置⽂件我们需要写到 kubez-ansible ⾥⾯的 ansible/group_vars/all.yml ⾥⾯
```yaml
# ################## 需要注意的是，这个文件中的配置的一些缩进与归属，上面的 YAML 我们需要放置到文件的顶头如下 ##################
charts:
  ......
  # 这里是我们要集成的应用的 Charts 需要传递的参数
  jenkins:
    name: jenkins
    namespace: "{{ jenkins_namespace }}"
    repository:
      name: "{{ jenkins_repo_name }}"
      url: "{{ jenkins_repo_url }}"
    chart:
      path: "{{ jenkins_path }}"
      version: "{{ jenkins_version }}"
    chart_extra_vars:
      persistence.storageClass: "{{ jenkins_storage_class }}"
      persistence.size: "{{ jenkins_storage_size }}"
      controller.adminPassword: "{{ initial_admin_password }}"
```

#### 1.3：同样在 all.yml 编译⼀个 Jenkins 可以配制项
```yaml
##################
# Jenkins Options
##################
enable_jenkins: "no"

# 这个地⽅就是前⾯ charts ⾥⾯的变量，默认是 chart ⾥⾯的，可供⽤户修改
jenkins_namespace: "{{ kubez_namespace }}"
jenkins_storage_class: managed-nfs-storage
jenkins_storage_size: "8Gi"
# The initial password for admin
initial_admin_password: "admin123456"
```

#### 1.4：后⾯还有⼀个配置，相当于打开这个项⽬，此配置需要配置在 enable_charts 内
```yaml
enable_charts:
  - name: jenkins
    enabled: "{{ enable_jenkins | bool }}"
```

#### 1.5：最终我们在 all.yml 中的配置是这样的
```yaml
##################
# Jenkins Options
##################
enable_jenkins: "no"

jenkins_namespace: "{{ kubez_namespace }}"
jenkins_storage_class: managed-nfs-storage
jenkins_storage_size: "8Gi"

# The initial password for admin
initial_admin_password: "admin123456"

# helm 仓库配置项
jenkins_repo_name: "{{ default_repo_name }}"
jenkins_repo_url: "{{ default_repo_url }}"
jenkins_path: pixiuio/jenkins
jenkins_version: 4.12.0

enable_charts:
  ...
  - name: jenkins
    enabled: "{{ enable_jenkins | bool }}"

charts:
  ...
  # 这里是我们要集成的应用的 Charts 需要传递的参数
  jenkins:
    name: jenkins
    namespace: "{{ jenkins_namespace }}"
    repository:
      name: "{{ jenkins_repo_name }}"
      url: "{{ jenkins_repo_url }}"
    chart:
      path: "{{ jenkins_path }}"
      version: "{{ jenkins_version }}"
    chart_extra_vars:
      persistence.storageClass: "{{ jenkins_storage_class }}"
      persistence.size: "{{ jenkins_storage_size }}"
      controller.adminPassword: "{{ initial_admin_password }}"
```

#### 1.6：我们最后的操作就是在 etc/kubez/globals.yml ⽂件中添加 Jenkins 的这个项目
```yaml
# 其实不难看出，我们是将 ansible/group_vars/all.yml 里面针对 Jenkins 的配置项放到了这里
# 如果我们去掉了注释，则证明我们需要集成 Jenkins
# 反之则不会集成 Jenkins

##################
# Jenkins Options
##################
#enable_jenkins: "no"
#jenkins_namespace: "{{ kubez_namespace }}"
#jenkins_storage_class: managed-nfs-storage
#jenkins_storage_size: 8Gi

# The initial password for admin
#initial_admin_password: admin123456

# helm 仓库配置项, 默认为 pixiu helm chart 仓库
#jenkins_repo_name: "{{ default_repo_name }}"
#jenkins_repo_url: "{{ default_repo_url }}"
#jenkins_path: pixiuio/jenkins
#jenkins_version: 4.12.0
```

### 2：若使用的是 YAML 文件部署项目
#### 2.1：如果是使⽤ YAML ⽂件部署项⽬，只需要将 YAML ⽂件添加到 ansible/roles/kubernetes/templates/xxxx.yml.j2
#### 2.2：然后在 all.yml ⽂件中添加以下配置
```yaml
# 以下为示例，请以真正集成项目为准

###############
# Jenkins Options
###############
enable_jenkins: "no"

kube_applications:
  ...
  - name: jenkins
    enabled: "{{ enable_jenkins | bool }}"
```

#### 2.3：最后也是需要将配置参数写入到  etc/kubez/globals.yml ⽂件中
```yaml
# 同 Helm 一致，若开启，则集成，若注释，则不集成

##################
# Jenkins Options
##################
#enable_jenkins: "no"
```
