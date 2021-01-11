## Kong for Kubernetes

[Kong for Kubernetes](https://github.com/Kong/kubernetes-ingress-controller)
is an open-source Ingress Controller for Kubernetes that offers
API management capabilities with a plugin architecture.

This chart bootstraps all the components needed to run Kong on a
[Kubernetes](http://kubernetes.io) cluster using the
[Helm](https://helm.sh) package manager.

## TL;DR;

```bash
$ helm repo add kong https://charts.konghq.com
$ helm repo update

# Helm 2
$ helm install kong/kong

# Helm 3
$ helm install kong/kong --generate-name --set ingressController.installCRDs=false
```

## Table of contents

- [Prerequisites](#prerequisites)
- [Helm 2 vs Helm 3](#important-helm-2-vs-helm-3)
- [Install](#install)
- [Uninstall](#uninstall)
- [Kong Enterprise](#kong-enterprise)
- [FAQs](#faqs)
- [Deployment Options](#deployment-options)
  - [Database](#database)
  - [Runtime package](#runtime-package)
  - [Configuration method](#configuration-method)
  - [Separate admin and proxy nodes](#separate-admin-and-proxy-nodes)
  - [Standalone controller nodes](#standalone-controller-nodes)
  - [Hybrid mode](#hybrid-mode)
  - [CRDs only](#crds-only)
  - [Example configurations](#example-configurations)
- [Configuration](#configuration)
  - [Kong Parameters](#kong-parameters)
    - [Kong Service Parameters](#kong-service-parameters)
  - [Ingress Controller Parameters](#ingress-controller-parameters)
  - [General Parameters](#general-parameters)
  - [The `env` section](#the-env-section)
- [Kong Enterprise Parameters](#kong-enterprise-parameters)
  - [Prerequisites](#prerequisites-1)
    - [Kong Enterprise License](#kong-enterprise-license)
    - [Kong Enterprise Docker registry access](#kong-enterprise-docker-registry-access)
  - [Service location hints](#service-location-hints)
  - [RBAC](#rbac)
  - [Sessions](#sessions)
  - [Email/SMTP](#emailsmtp)
- [Changelog](https://github.com/Kong/charts/blob/main/charts/kong/CHANGELOG.md)
- [Upgrading](https://github.com/Kong/charts/blob/main/charts/kong/UPGRADE.md)
- [Seeking help](#seeking-help)

## Prerequisites

- Kubernetes 1.12+
- PV provisioner support in the underlying infrastructure if persistence
  is needed for Kong datastore.

## Important: Helm 2 vs Helm 3

Custom Resource Definitions (CRDs) are handled differently in Helm 2 vs Helm 3.

#### Helm 2

If you want CRDs to be installed,
make sure `ingressController.installCRDs` is set to `true` (the default value).
Set this value to `false` to skip installing CRDs.

#### Helm 3

Make sure `ingressController.installCRDs` is set to `false`,
note that the default is `true`.
You can do so either by passing in a custom `values.yaml`
(`-f` when running helm)
or by passing `--set ingressController.installCRDs=false`
at the command line.

**If you do not set this value to `false`, the helm chart will not install correctly.**

Use helm CLI flag `--skip-crds` with `helm install` if you want to skip
CRD creation while creating a release.

## Install

To install Kong:

```bash
$ helm repo add kong https://charts.konghq.com
$ helm repo update

# Helm 2
$ helm install kong/kong

# Helm 3
$ helm install kong/kong --generate-name --set ingressController.installCRDs=false
```

## Uninstall

To uninstall/delete a Helm release `my-release`:

```bash
$ helm delete my-release
```

The command removes all the Kubernetes components associated with the
chart and deletes the release.

> **Tip**: List all releases using `helm list`

## FAQs

Please read the
[FAQs](https://github.com/Kong/charts/blob/main/charts/kong/FAQs.md)
document.

## Kong Enterprise

If using Kong Enterprise, several additional steps are necessary before
installing the chart:

- Set `enterprise.enabled` to `true` in `values.yaml` file.
- Update values.yaml to use a Kong Enterprise image.
- Satisfy the two  prerequsisites below for
  [Enterprise License](#kong-enterprise-license) and
  [Enterprise Docker Registry](#kong-enterprise-docker-registry-access).
- (Optional) [set a `password` environment variable](#rbac) to create the
  initial super-admin. Though not required, this is recommended for users that
  wish to use RBAC, as it cannot be done after initial setup.

Once you have these set, it is possible to install Kong Enterprise.

Please read through
[Kong Enterprise considerations](#kong-enterprise-parameters)
to understand all settings that are enterprise specific.

## Deployment Options

Kong is a highly configurable piece of software that can be deployed
in a number of different ways, depending on your use-case.

All combinations of various runtimes, databases and configuration methods are
supported by this Helm chart.
The recommended approach is to use the Ingress Controller based configuration
along-with DB-less mode.

Following sections detail on various high-level architecture options available:

### Database

Kong can run with or without a database (DB-less).
By default, this chart installs Kong without a database.

Although Kong can run with Postgres and Cassandra, the recommended database,
if you would like to use one, is Postgres for Kubernetes installations.
If your use-case warrants Cassandra, you should run the Cassandra cluster
outside of Kubernetes.

The database to use for Kong can be controlled via the `env.database` parameter.
For more details, please read the [env](#the-env-section) section.

Furthermore, this chart allows you to bring your own database that you manage
or spin up a new Postgres instance using the `postgres.enabled` parameter.

> Cassandra deployment via a sub-chart was previously supported but
the support has now been dropped due to stability issues.
You can still deploy Cassandra on your own and configure Kong to use
that via the `env.database` parameter.

#### DB-less  deployment

When deploying Kong in DB-less mode(`env.database: "off"`)
and without the Ingress Controller(`ingressController.enabled: false`),
you have to provide a declarative configuration for Kong to run.
The configuration can be provided using an existing ConfigMap
(`dblessConfig.configMap`) or or the whole configuration can be put into the
`values.yaml` file for deployment itself, under the `dblessConfig.config`
parameter. See the example configuration in the default values.yaml
for more details.

### Runtime package

There are three different packages of Kong that are available:

- **Kong Gateway**\
  This is the [Open-Source](https://github.com/kong/kong) offering. It is a
  full-blown API Gateway and Ingress solution with a wide-array of functionality.
  When Kong Gateway is combined with the Ingress based configuration method,
  you get Kong for Kubernetes. This is the default deployment for this Helm
  Chart.
- **Kong Enterprise K8S**\
  This package builds up on top of the Open-Source Gateway and bundles in all
  the Enterprise-only plugins as well.
  When Kong Enterprise K8S is combined with the Ingress based
  configuration method, you get Kong for Kubernetes Enterprise.
  This package also comes with 24x7 support from Kong Inc.
- **Kong Enterprise**\
  This is the full-blown Enterprise package which packs with itself all the
  Enterprise functionality like Manager, Portal, Vitals, etc.
  This package can't be run in DB-less mode.

The package to run can be changed via `image.repository` and `image.tag`
parameters. If you would like to run the Enterprise package, please read
the [Kong Enterprise Parameters](#kong-enterprise-parameters) section.

### Configuration method

Kong can be configured via two methods:
- **Ingress and CRDs**\
  The configuration for Kong is done via `kubectl` and Kubernetes-native APIs.
  This is also known as Kong Ingress Controller or Kong for Kubernetes and is
  the default deployment pattern for this Helm Chart. The configuration
  for Kong is managed via Ingress and a few
  [Custom Resources](https://github.com/Kong/kubernetes-ingress-controller/blob/main/docs/concepts/custom-resources.md).
  For more details, please read the
  [documentation](https://github.com/Kong/kubernetes-ingress-controller/tree/main/docs)
  on Kong Ingress Controller.
  To configure and fine-tune the controller, please read the
  [Ingress Controller Parameters](#ingress-controller-parameters) section.
- **Admin API**\
  This is the traditional method of running and configuring Kong.
  By default, the Admin API of Kong is not exposed as a Service. This
  can be controlled via `admin.enabled` and `env.admin_listen` parameters.

### Separate admin and proxy nodes

*Note: although this section is titled "Separate admin and proxy nodes", this
split release technique is generally applicable to any deployment with
different types of Kong nodes. Separating Admin API and proxy nodes is one of
the more common use cases for splitting across multiple releases, but you can
also split releases for hybrid mode CP/DP nodes, split proxy and Developer
Portal nodes, etc.*

Users may wish to split their Kong deployment into multiple instances that only
run some of Kong's services (i.e. you run `helm install` once for every
instance type you wish to create).

To disable Kong services on an instance, you should set `SVC.enabled`,
`SVC.http.enabled`, `SVC.tls.enabled`, and `SVC.ingress.enabled` all to
`false`, where `SVC` is `proxy`, `admin`, `manager`, `portal`, or `portalapi`.

The standard chart upgrade automation process assumes that there is only a
single Kong release in the Kong cluster, and runs both `migrations up` and
`migrations finish` jobs. To handle clusters split across multiple releases,
you should:
1. Upgrade one of the releases with `helm upgrade RELEASENAME -f values.yaml
   --set migrations.preUpgrade=true --set migrations.postUpgrade=false`.
2. Upgrade all but one of the remaining releases with `helm upgrade RELEASENAME
   -f values.yaml --set migrations.preUpgrade=false --set
   migrations.postUpgrade=false`.
3. Upgrade the final release with `helm upgrade RELEASENAME -f values.yaml
   --set migrations.preUpgrade=false --set migrations.postUpgrade=true`.

This ensures that all instances are using the new Kong package before running
`kong migrations finish`.

Users should note that Helm supports supplying multiple values.yaml files,
allowing you to separate shared configuration from instance-specific
configuration. For example, you may have a shared values.yaml that contains
environment variables and other common settings, and then several
instance-specific values.yamls that contain service configuration only. You can
then create releases with:

```bash
helm install proxy-only -f shared-values.yaml -f only-proxy.yaml kong/kong
helm install admin-only -f shared-values.yaml -f only-admin.yaml kong/kong
```

### Standalone controller nodes

The chart can deploy releases that contain the controller only, with no Kong
container, by setting `deployment.kong.enabled: false` in values.yaml. There
are several controller settings that must be populated manually in this
scenario and several settings that are useful when using multiple controllers:

* `ingressController.env.kong_admin_url` must be set to the Kong Admin API URL.
  If the Admin API is exposed by a service in the cluster, this should look
  something like `https://my-release-kong-admin.kong-namespace.svc:8444`
* `ingressController.env.publish_service` must be set to the Kong proxy
  service, e.g. `namespace/my-release-kong-proxy`.
* `ingressController.ingressClass` should be set to a different value for each
  instance of the controller.
* `ingressController.env.admin_filter_tag` should be set to a different value
  for each instance of the controller.
* If using Kong Enterprise, `ingressController.env.kong_workspace` can
  optionally create configuration in a workspace other than `default`.

Standalone controllers require a database-backed Kong instance, as DB-less mode
requires that a single controller generate a complete Kong configuration.

### Hybrid mode

Kong supports [hybrid mode
deployments](https://docs.konghq.com/2.0.x/hybrid-mode/) as of Kong 2.0.0 and
[Kong Enterprise 2.1.0](https://docs.konghq.com/enterprise/2.1.x/deployment/hybrid-mode/).
These deployments split Kong nodes into control plane (CP) nodes, which provide
the admin API and interact with the database, and data plane (DP) nodes, which
provide the proxy and receive configuration from control plane nodes.

You can deploy hybrid mode Kong clusters by [creating separate releases for each node
type](#separate-admin-and-proxy-nodes), i.e. use separate control and data
plane values.yamls that are then installed separately. The [control
plane](#control-plane-node-configuration) and [data
plane](#data-plane-node-configuration) configuration sections below cover the
values.yaml specifics for each.

Cluster certificates are not generated automatically. You must [create a
certificate and key pair](#certificates) for intra-cluster communication.

#### Certificates

Hybrid mode uses TLS to secure the CP/DP node communication channel, and
requires certificates for it. You can generate these either using `kong hybrid
gen_cert` on a local Kong installation or using OpenSSL:

```bash
openssl req -new -x509 -nodes -newkey ec:<(openssl ecparam -name secp384r1) \
  -keyout /tmp/cluster.key -out /tmp/cluster.crt \
  -days 1095 -subj "/CN=kong_clustering"
```

You must then place these certificates in a Secret:

```bash
kubectl create secret tls kong-cluster-cert --cert=/tmp/cluster.crt --key=/tmp/cluster.key
```

#### Control plane node configuration

You must configure the control plane nodes to mount the certificate secret on
the container filesystem is serve it from the cluster listen. In values.yaml:

```yaml
secretVolumes:
- kong-cluster-cert
```

```yaml
env:
  role: control_plane
  cluster_cert: /etc/secrets/kong-cluster-cert/tls.crt
  cluster_cert_key: /etc/secrets/kong-cluster-cert/tls.key
```

Furthermore, you must enable the cluster listen and Kubernetes Service, and
should typically disable the proxy:

```yaml
cluster:
  enabled: true
  tls:
    enabled: true
    servicePort: 8005
    containerPort: 8005

proxy:
  enabled: false
```

Enterprise users with Vitals enabled must also enable the cluster telemetry
service:

```yaml
clustertelemetry:
  enabled: true
  tls:
    enabled: true
    servicePort: 8006
    containerPort: 8006
```

If using the ingress controller, you must also specify the DP proxy service as
its publish target to keep Ingress status information up to date:

```
ingressController:
  env:
    publish_service: hybrid/example-release-data-kong-proxy
```

Replace `hybrid` with your DP nodes' namespace and `example-release-data` with
the name of the DP release.

#### Data plane node configuration

Data plane configuration also requires the certificate and `role`
configuration, and the database should always be set to `off`. You must also
trust the cluster certificate and indicate what hostname/port Kong should use
to find control plane nodes.

Though not strictly required, you should disable the admin service (it will not
work on DP nodes anyway, but should be disabled to avoid creating an invalid
Service resource).

```yaml
secretVolumes:
- kong-cluster-cert
```

```yaml
admin:
  enabled: false
```

```yaml
env:
  role: data_plane
  database: off
  cluster_cert: /etc/secrets/kong-cluster-cert/tls.crt
  cluster_cert_key: /etc/secrets/kong-cluster-cert/tls.key
  lua_ssl_trusted_certificate: /etc/secrets/kong-cluster-cert/tls.crt
  cluster_control_plane: control-plane-release-name-kong-cluster.hybrid.svc.cluster.local:8005
  cluster_telemetry_endpoint: control-plane-release-name-kong-clustertelemetry.hybrid.svc.cluster.local:8006 # Enterprise-only
```

Note that the `cluster_control_plane` value will differ depending on your
environment. `control-plane-release-name` will change to your CP release name,
`hybrid` will change to whatever namespace it resides in. See [Kubernetes'
documentation on Service
DNS](https://kubernetes.io/docs/concepts/services-networking/dns-pod-service/)
for more detail.

### CRDs only

For Helm 2 installations, CRDs are managed as part of a release, and are
deleted if the release is. This can cause issues for clusters with multiple
Kong installations, as one release must remain in place for the rest to
function. To avoid this, you can create a CRD-only release by setting
`deployment.kong.enabled: false` and `ingressController.enabled: false`.

On Helm 3, CRDs are created if necessary, but are not managed along with the
release. Releases can be deleted without affecting CRDs; CRDs are only removed
if you delete them manually.

### Sidecar Containers

The chart can deploy additional containers along with the Kong and Ingress
Controller containers, sometimes referred to as "sidecar containers".  This can
be useful to include network proxies or logging services along with Kong.  The
`deployment.sidecarContainers` field in values.yaml takes an array of objects
that get appended as-is to the existing `spec.template.spec.containers` array
in the Kong deployment resource.

### Example configurations

Several example values.yaml are available in the
[example-values](https://github.com/Kong/charts/blob/main/charts/kong/example-values/)
directory.

## Configuration

### Kong parameters

| Parameter                          | Description                                                                           | Default             |
| ---------------------------------- | ------------------------------------------------------------------------------------- | ------------------- |
| image.repository                   | Kong image                                                                            | `kong`              |
| image.tag                          | Kong image version                                                                    | `2.0`               |
| image.pullPolicy                   | Image pull policy                                                                     | `IfNotPresent`      |
| image.pullSecrets                  | Image pull secrets                                                                    | `null`              |
| replicaCount                       | Kong instance count. It has no effect when `autoscaling.enabled` is set to true         | `1`                 |
| plugins                            | Install custom plugins into Kong via ConfigMaps or Secrets                            | `{}`                |
| env                                | Additional [Kong configurations](https://getkong.org/docs/latest/configuration/)      |                     |
| migrations.preUpgrade              | Run "kong migrations up" jobs                                                         | `true`              |
| migrations.postUpgrade             | Run "kong migrations finish" jobs                                                     | `true`              |
| migrations.annotations             | Annotations for migration job pods                                                    | `{"sidecar.istio.io/inject": "false", "kuma.io/sidecar-injection": "disabled"}` |
| migrations.jobAnnotations          | Additional annotations for migration jobs                                             | `{}`                |
| waitImage.repository               | Image used to wait for database to become ready                                       | `bash`              |
| waitImage.tag                      | Tag for image used to wait for database to become ready                               | `5`                 |
| waitImage.pullPolicy               | Wait image pull policy                                                                | `IfNotPresent`      |
| postgresql.enabled                 | Spin up a new postgres instance for Kong                                              | `false`             |
| dblessConfig.configMap             | Name of an existing ConfigMap containing the `kong.yml` file. This must have the key `kong.yml`.| `` |
| dblessConfig.config                | Yaml configuration file for the dbless (declarative) configuration of Kong | see in `values.yaml`    |

#### Kong Service Parameters

The various `SVC.*` parameters below are common to the various Kong services
(the admin API, proxy, Kong Manger, the Developer Portal, and the Developer
Portal API) and define their listener configuration, K8S Service properties,
and K8S Ingress properties. Defaults are listed only if consistent across the
individual services: see values.yaml for their individual default values.

`SVC` below can be substituted with each of:
* `proxy`
* `admin`
* `manager`
* `portal`
* `portalapi`
* `cluster`
* `clustertelemetry`
* `status`

`status` is intended for internal use within the cluster. Unlike other
services it cannot be exposed externally, and cannot create a Kubernetes
service or ingress. It supports the settings under `SVC.http` and `SVC.tls`
only.

`cluster` is used on hybrid mode control plane nodes. It does not support the
`SVC.http.*` settings (cluster communications must be TLS-only) or the
`SVC.ingress.*` settings (cluster communication requires TLS client
authentication, which cannot pass through an ingress proxy). `clustertelemetry`
is similar, and used when Vitals is enabled on Kong Enterprise control plane
nodes.

| Parameter                          | Description                                                                           | Default             |
| ---------------------------------- | ------------------------------------------------------------------------------------- | ------------------- |
| SVC.enabled                        | Create Service resource for SVC (admin, proxy, manager, etc.)                         |                     |
| SVC.http.enabled                   | Enables http on the service                                                           |                     |
| SVC.http.servicePort               | Service port to use for http                                                          |                     |
| SVC.http.containerPort             | Container port to use for http                                                        |                     |
| SVC.http.nodePort                  | Node port to use for http                                                             |                     |
| SVC.http.hostPort                  | Host port to use for http                                                             |                     |
| SVC.http.parameters                | Array of additional listen parameters                                                 | `[]`                |
| SVC.tls.enabled                    | Enables TLS on the service                                                            |                     |
| SVC.tls.containerPort              | Container port to use for TLS                                                         |                     |
| SVC.tls.servicePort                | Service port to use for TLS                                                           |                     |
| SVC.tls.nodePort                   | Node port to use for TLS                                                              |                     |
| SVC.tls.hostPort                   | Host port to use for TLS                                                              |                     |
| SVC.tls.overrideServiceTargetPort  | Override service port to use for TLS without touching Kong containerPort              |                     |
| SVC.tls.parameters                 | Array of additional listen parameters                                                 | `["http2"]`         |
| SVC.type                           | k8s service type. Options: NodePort, ClusterIP, LoadBalancer                          |                     |
| SVC.clusterIP                      | k8s service clusterIP                                                                 |                     |
| SVC.loadBalancerSourceRanges       | Limit service access to CIDRs if set and service type is `LoadBalancer`               | `[]`                |
| SVC.loadBalancerIP                 | Reuse an existing ingress static IP for the service                                   |                     |
| SVC.externalIPs                    | IPs for which nodes in the cluster will also accept traffic for the servic            | `[]`                |
| SVC.externalTrafficPolicy          | k8s service's externalTrafficPolicy. Options: Cluster, Local                          |                     |
| SVC.ingress.enabled                | Enable ingress resource creation (works with SVC.type=ClusterIP)                      | `false`             |
| SVC.ingress.tls                    | Name of secret resource, containing TLS secret                                        |                     |
| SVC.ingress.hostname               | Ingress hostname                                                                      | `""`                |
| SVC.ingress.path                   | Ingress path.                                                                         | `/`                 |
| SVC.ingress.annotations            | Ingress annotations. See documentation for your ingress controller for details        | `{}`                |
| SVC.annotations                    | Service annotations                                                                   | `{}`                |

#### Stream listens

The proxy configuration additionally supports creating stream listens. These
are configured using an array of objects under `proxy.stream`:

| Parameter                          | Description                                                                           | Default             |
| ---------------------------------- | ------------------------------------------------------------------------------------- | ------------------- |
| containerPort                      | Container port to use for a stream listen                                             |                     |
| servicePort                        | Service port to use for a stream listen                                               |                     |
| nodePort                           | Node port to use for a stream listen                                                  |                     |
| hostPort                           | Host port to use for a stream listen                                                  |                     |
| parameters                         | Array of additional listen parameters                                                 | `[]`                |

### Ingress Controller Parameters

All of the following properties are nested under the `ingressController`
section of `values.yaml` file:

| Parameter                          | Description                                                                           | Default                                                                      |
| ---------------------------------- | ------------------------------------------------------------------------------------- | ---------------------------------------------------------------------------- |
| enabled                            | Deploy the ingress controller, rbac and crd                                           | true                                                                         |
| image.repository                   | Docker image with the ingress controller                                              | kong-docker-kubernetes-ingress-controller.bintray.io/kong-ingress-controller |
| image.tag                          | Version of the ingress controller                                                     | 0.9.1                                                                        |
| readinessProbe                     | Kong ingress controllers readiness probe                                              |                                                                              |
| livenessProbe                      | Kong ingress controllers liveness probe                                               |                                                                              |
| installCRDs                        | Create CRDs. **FOR HELM3, MAKE SURE THIS VALUE IS SET TO `false`.**  Regardless of value of this, Helm v3+ will install the CRDs if those are not present already. Use `--skip-crds` with `helm install` if you want to skip CRD creation.                 | true                                                                         |
| serviceAccount.create              | Create Service Account for ingress controller                                         | true
| serviceAccount.name                | Use existing Service Account, specify its name                                        | ""
| serviceAccount.annotations         | Annotations for Service Account                                                       | {}
| env                                | Specify Kong Ingress Controller configuration via environment variables               |                                                                              |
| ingressClass                       | The ingress-class value for controller                                                | kong                                                                         |
| args                               | List of ingress-controller cli arguments                                              | []                                                                           |
| admissionWebhook.enabled           | Whether to enable the validating admission webhook                                    | false                                                                        |
| admissionWebhook.failurePolicy     | How unrecognized errors from the admission endpoint are handled (Ignore or Fail)      | Fail                                                                         |
| admissionWebhook.port              | The port the ingress controller will listen on for admission webhooks                 | 8080                                                                         |

For a complete list of all configuration values you can set in the
`env` section, please read the Kong Ingress Controller's
[configuration document](https://github.com/Kong/kubernetes-ingress-controller/blob/main/docs/references/cli-arguments.md).

### General Parameters

| Parameter                          | Description                                                                           | Default             |
| ---------------------------------- | ------------------------------------------------------------------------------------- | ------------------- |
| namespace                          | Namespace to deploy chart resources                                                   |                     |
| deployment.kong.enabled            | Enable or disable deploying Kong                                                      | `true`              |
| autoscaling.enabled                | Set this to `true` to enable autoscaling                                              | `false`             |
| autoscaling.minReplicas            | Set minimum number of replicas                                                        | `2`                 |
| autoscaling.maxReplicas            | Set maximum number of replicas                                                        | `5`                 |
| autoscaling.targetCPUUtilizationPercentage | Target Percentage for when autoscaling takes affect. Only used if cluster doesnt support `autoscaling/v2beta2` | `80`  |
| autoscaling.metrics                | metrics used for autoscaling for clusters that support autoscaling/v2beta2`           | See [values.yaml](values.yaml) |
| updateStrategy                     | update strategy for deployment                                                        | `{}`                |
| readinessProbe                     | Kong readiness probe                                                                  |                     |
| livenessProbe                      | Kong liveness probe                                                                   |                     |
| lifecycle                          | Proxy container lifecycle hooks                                                       | see `values.yaml`   |
| affinity                           | Node/pod affinities                                                                   |                     |
| nodeSelector                       | Node labels for pod assignment                                                        | `{}`                |
| deploymentAnnotations              | Annotations to add to deployment                                                      |  see `values.yaml`  |
| podAnnotations                     | Annotations to add to each pod                                                        | `{}`                |
| podLabels                          | Labels to add to each pod                                                             | `{}`                |
| resources                          | Pod resource requests & limits                                                        | `{}`                |
| tolerations                        | List of node taints to tolerate                                                       | `[]`                |
| podDisruptionBudget.enabled        | Enable PodDisruptionBudget for Kong                                                   | `false`             |
| podDisruptionBudget.maxUnavailable | Represents the minimum number of Pods that can be unavailable (integer or percentage) | `50%`               |
| podDisruptionBudget.minAvailable   | Represents the number of Pods that must be available (integer or percentage)          |                     |
| podSecurityPolicy.enabled          | Enable podSecurityPolicy for Kong                                                     | `false`             |
| podSecurityPolicy.spec             | Collection of [PodSecurityPolicy settings](https://kubernetes.io/docs/concepts/policy/pod-security-policy/#what-is-a-pod-security-policy) | |
| priorityClassName                  | Set pod scheduling priority class for Kong pods                                       | `""`                |
| secretVolumes                      | Mount given secrets as a volume in Kong container to override default certs and keys. | `[]`                |
| securityContext                    | Set the securityContext for Kong Pods                                                 | `{}`                |
| serviceMonitor.enabled             | Create ServiceMonitor for Prometheus Operator                                         | `false`             |
| serviceMonitor.interval            | Scraping interval                                                                     | `30s`               |
| serviceMonitor.namespace           | Where to create ServiceMonitor                                                        |                     |
| serviceMonitor.labels              | ServiceMonitor labels                                                                 | `{}`                |
| serviceMonitor.targetLabels        | ServiceMonitor targetLabels                                                           | `{}`                |
| extraConfigMaps                    | ConfigMaps to add to mounted volumes                                                  | `[]`                |
| extraSecrets                       | Secrets to add to mounted volumes                                                     | `[]`                |


#### The `env` section

The `env` section can be used to configured all properties of Kong.
Any key value put under this section translates to environment variables
used to control Kong's configuration. Every key is prefixed with `KONG_`
and upper-cased before setting the environment variable.

Furthermore, all `kong.env` parameters can also accept a mapping instead of a
value to ensure the parameters can be set through configmaps and secrets.

An example:

```yaml
kong:
  env:                       # load PG password from a secret dynamically
     pg_user: kong
     pg_password:
       valueFrom:
         secretKeyRef:
            key: kong
            name: postgres
  nginx_worker_processes: "2"
```

For complete list of Kong configurations please check the
[Kong configuration docs](https://docs.konghq.com/latest/configuration).

> **Tip**: You can use the default [values.yaml](values.yaml)

## Kong Enterprise Parameters

### Overview

Kong Enterprise requires some additional configuration not needed when using
Kong Open-Source. To use Kong Enterprise, at the minimum,
you need to do the following:

- Set `enterprise.enabled` to `true` in `values.yaml` file.
- Update values.yaml to use a Kong Enterprise image.
- Satisfy the two prerequsisites below for Enterprise License and
  Enterprise Docker Registry.
- (Optional) [set a `password` environment variable](#rbac) to create the
  initial super-admin. Though not required, this is recommended for users that
  wish to use RBAC, as it cannot be done after initial setup.

Once you have these set, it is possible to install Kong Enterprise,
but please make sure to review the below sections for other settings that
you should consider configuring before installing Kong.

Some of the more important configuration is grouped in sections
under the `.enterprise` key in values.yaml, though most enterprise-specific
configuration can be placed under the `.env` key.

### Prerequisites

#### Kong Enterprise License

All Kong Enterprise deployments require a license. If you do not have a copy
of yours, please contact Kong Support. Once you have it, you will need to
store it in a Secret:

```bash
$ kubectl create secret generic kong-enterprise-license --from-file=license=./license.json
```

Set the secret name in `values.yaml`, in the `.enterprise.license_secret` key.
Please ensure the above secret is created in the same namespace in which
Kong is going to be deployed.

#### Kong Enterprise Docker registry access

Next, we need to setup Docker credentials in order to allow Kubernetes
nodes to pull down Kong Enterprise Docker images, which are hosted in a private
registry.

You should received credentials to log into https://bintray.com/kong after
purchasing Kong Enterprise. After logging in, you can retrieve your API key
from \<your username\> \> Edit Profile \> API Key. Use this to create registry
secrets:

```bash
$ kubectl create secret docker-registry kong-enterprise-edition-docker \
    --docker-server=kong-docker-kong-enterprise-edition-docker.bintray.io \
    --docker-username=<your-bintray-username@kong> \
    --docker-password=<your-bintray-api-key>
secret/kong-enterprise-edition-docker created
```

Set the secret names in `values.yaml` in the `image.pullSecrets` section.
Again, please ensure the above secret is created in the same namespace in which
Kong is going to be deployed.

### Service location hints

Kong Enterprise add two GUIs, Kong Manager and the Kong Developer Portal, that
must know where other Kong services (namely the admin and files APIs) can be
accessed in order to function properly. Kong's default behavior for attempting
to locate these absent configuration is unlikely to work in common Kubernetes
environments. Because of this, you should set each of `admin_gui_url`,
`admin_api_uri`, `proxy_url`, `portal_api_url`, `portal_gui_host`, and
`portal_gui_protocol` under the `.env` key in values.yaml to locations where
each of their respective services can be accessed to ensure that Kong services
can locate one another and properly set CORS headers. See the
[Property Reference documentation](https://docs.konghq.com/enterprise/latest/property-reference/)
for more details on these settings.

### RBAC

You can create a default RBAC superuser when initially running `helm install`
by setting a `password` environment variable under `env` in values.yaml. It
should be a reference to a secret key containing your desired password. This
will create a `kong_admin` admin whose token and basic-auth password match the
value in the secret. For example:

```yaml
env:
 password:
   valueFrom:
     secretKeyRef:
        name: kong-enterprise-superuser-password
        key: password
```

If using the ingress controller, it needs access to the token as well, by
specifying `kong_admin_token` in its environment variables:

```yaml
ingressController:
  env:
   kong_admin_token:
     valueFrom:
       secretKeyRef:
          name: kong-enterprise-superuser-password
          key: password
```

Although the above examples both use the initial super-admin, we recommend
[creating a less-privileged RBAC user](https://docs.konghq.com/enterprise/latest/kong-manager/administration/rbac/add-user/)
for the controller after installing. It needs at least workspace admin
privileges in its workspace (`default` by default, settable by adding a
`workspace` variable under `ingressController.env`). Once you create the
controller user, add its token to a secret and update your `kong_admin_token`
variable to use it. Remove the `password` variable from Kong's environment
variables and the secret containing the super-admin token after.

### Sessions

Login sessions for Kong Manager and the Developer Portal make use of
[the Kong Sessions plugin](https://docs.konghq.com/enterprise/latest/kong-manager/authentication/sessions).
When configured via values.yaml, their configuration must be stored in Secrets,
as it contains an HMAC key.

Kong Manager's session configuration must be configured via values.yaml,
whereas this is optional for the Developer Portal on versions 0.36+. Providing
Portal session configuration in values.yaml provides the default session
configuration, which can be overriden on a per-workspace basis.

```
$ cat admin_gui_session_conf
{"cookie_name":"admin_session","cookie_samesite":"off","secret":"admin-secret-CHANGEME","cookie_secure":true,"storage":"kong"}
$ cat portal_session_conf
{"cookie_name":"portal_session","cookie_samesite":"off","secret":"portal-secret-CHANGEME","cookie_secure":true,"storage":"kong"}
$ kubectl create secret generic kong-session-config --from-file=admin_gui_session_conf --from-file=portal_session_conf
secret/kong-session-config created
```
The exact plugin settings may vary in your environment. The `secret` should
always be changed for both configurations.

After creating your secret, set its name in values.yaml in
`.enterprise.rbac.session_conf_secret`. If you create a Portal configuration,
add it at `env.portal_session_conf` using a secretKeyRef.

### Email/SMTP

Email is used to send invitations for
[Kong Admins](https://docs.konghq.com/enterprise/latest/kong-manager/networking/email)
and [Developers](https://docs.konghq.com/enterprise/latest/developer-portal/configuration/smtp).

Email invitations rely on setting a number of SMTP settings at once. For
convenience, these are grouped under the `.enterprise.smtp` key in values.yaml.
Setting `.enterprise.smtp.disabled: true` will set `KONG_SMTP_MOCK=on` and
allow Admin/Developer invites to proceed without sending email. Note, however,
that these have limited functionality without sending email.

If your SMTP server requires authentication, you must provide the `username` and `smtp_password_secret` keys under `.enterprise.smtp.auth`. `smtp_password_secret` must be a Secret containing an `smtp_password` key whose value is your SMTP password.

By default, SMTP uses `AUTH` `PLAIN` when you provide credentials. If your provider requires `AUTH LOGIN`, set `smtp_auth_type: login`.

## Seeking help

If you run into an issue, bug or have a question, please reach out to the Kong
community via [Kong Nation](https://discuss.konghq.com).
Please do not open issues in [this](https://github.com/helm/charts) repository
as the maintainers will not be notified and won't respond.
