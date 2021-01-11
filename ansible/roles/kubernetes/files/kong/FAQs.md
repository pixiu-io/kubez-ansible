# Frequently Asked Questions (FAQs)

#### Kong fails to start after `helm upgrade` when Postgres is used. What do I do?

You may be running into this issue: https://github.com/helm/charts/issues/12575.
This issue is caused due to: https://github.com/helm/helm/issues/3053.

The problem that happens is that Postgres database has the old password but
the new secret has a different password, which is used by Kong, and password
based authentication fails.

The solution to the problem is to specify a password to the `postgresql` chart.
This is to ensure that the password is not generated randomly but is set to
the same one that is user-provided on each upgrade.

#### Kong fails to start on a fresh installation with Postgres. What do I do?

Please make sure that there is no `PersistentVolumes` present from a previous
release. If there are, it can lead to data or passwords being out of sync
and result in connection issues.

A simple way to find out is to use the following command:

```
kubectl get pv -n <your-namespace>
```

And then based on the `AGE` column, determine if you have an old volume.
If you do, then please delete the release, delete the volume, and then
do a fresh installation. PersistentVolumes can remain in the cluster even if
you delete the namespace itself (the namespace in which they were present).

#### Upgrading a release fails due to missing ServiceAccount

When upgrading a release, some configuration changes result in this error:

```
Error creating: pods "releasename-kong-pre-upgrade-migrations-" is forbidden: error looking up service account releasename-kong: serviceaccount "releasename-kong" not found
```

Enabling the ingress controller or PodSecurityPolicy requires that the Kong
chart also create a ServiceAccount. When upgrading from a configuration that
previously had neither of these features enabled, the pre-upgrade-migrations
Job attempts to use this ServiceAccount before it is created. It is [not
possible to easily handle this case automatically](https://github.com/Kong/charts/pull/31).

Users encountering this issue should temporarily modify their
[pre-upgrade-migrations template](https://github.com/Kong/charts/blob/main/charts/kong/templates/migrations-pre-upgrade.yaml),
adding the following at the bottom:

```
{{ if or .Values.podSecurityPolicy.enabled (and .Values.ingressController.enabled .Values.ingressController.serviceAccount.create) -}}
---
apiVersion: v1
kind: ServiceAccount
metadata:
  name: {{ template "kong.serviceAccountName" . }}
  namespace: {{ template "kong.namespace" . }}
  annotations:
    "helm.sh/hook": pre-upgrade
    "helm.sh/hook-delete-policy": before-hook-creation,hook-succeeded
  labels:
    {{- include "kong.metaLabels" . | nindent 4 }}
{{- end -}}
```

Upgrading with this in place will create a temporary service account before
creating the actual service account. After this initial upgrade, users must
revert to the original pre-upgrade migrations template, as leaving the
temporary ServiceAccount template in place will [cause permissions issues on
subsequent upgrades](https://github.com/Kong/charts/issues/30).

#### Running "helm upgrade" fails because of old init-migrations Job

When running `helm upgrade`, the upgrade fails and Helm reports an error
similar to the following:

```
Error: UPGRADE FAILED: cannot patch "RELEASE-NAME-kong-init-migrations" with
kind Job: Job.batch "RELEASE-NAME-kong-init-migrations" is invalid ... field
is immutable
```

This occurs if a `RELEASE-NAME-kong-init-migrations` Job is left over from a
previous `helm install` or `helm upgrade`. Deleting it with
`kubectl delete job RELEASE-NAME-kong-init-migrations` will allow the upgrade
to proceed. Chart versions greater than 1.5.0 delete the job automatically.
