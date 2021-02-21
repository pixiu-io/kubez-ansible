# HPA operator Chart

HPA operator (https://github.com/banzaicloud/hpa-operator) takes care of creating, deleting, updating HPA, with other words keeping in sync with your deployment annotations.

## Installing the Chart

To install the chart:

```
$ helm install banzaicloud-stable/hpa-operator
```

Installing chart with enabled PodSecurityPolicy:
```
$ helm install banzaicloud-stable/hpa-operator --set pspEnabled=true --set kube-metrics-adapter.pspEnabled=true
```

## Configuration

The following table lists the configurable parameters of the chart and their default values.

| Parameter                       | Description                                                                     | Default                                     |
| ------------------------------- | ------------------------------------------------------------------------------- | --------------------------------------------|
| `affinity`                      | Node affinity                                                                   | `{}`                                        |
| `image.repository`              | Image repository                                                                | `banzaicloud/hpa-operator`          |
| `image.tag`                     | Image tag                                                                       | `0.2.0`                                     |
| `image.pullPolicy`              | Image pull policy                                                               | `IfNotPresent`                              |
| `image.pullSecrets`             | Image pull secrets                                                              | `[]`                                        |
| `nodeSelector`                  | Node labels for pod assignment                                                  | `{}`                                        |
| `metrics-server.enabled`                  | Install Metrics Server chart                                                  | `false`                                        |
| `kube-metrics-adapter.enabled`                  | Install Kube Metrics Adapter chart                                                | `true`                                        |
| `rbac.enabled`                   | If true, install default RBAC roles and bindings                                            | `true`                                      |
| `monitoring.enabled`                   | If true, install Service Monitor resource for Prometheus monitoring                                          | `false`                                      |
| `resources`                     | CPU/Memory resource requests/limits                                             | `{}`                                        |                                                                                                        
| `serviceAccount.create`         | If true, create & use Service account                                            | `true`                                      |
| `serviceAccount.name`           | If not set and create is true, a name is generated using the fullname template  | ``                                          |
| `rbac.psp.enabled`                    | enabel PSP resources                                                            | false                                       |
Specify each parameter using the `--set key=value[,key=value]` argument to `helm install`. For example,

```console
$ helm install --name my-release \
  --set logLevel=1 \
 banzaicloud-stable/hpa-operator
```

Alternatively, a YAML file that specifies the values for the above parameters can be provided while installing the chart. For example,

```console
$ helm install --name my-release -f values.yaml banzaicloud-stable/hpa-operator
```



