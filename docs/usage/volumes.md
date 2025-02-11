<!-- markdownlint-disable MD041 -->

# Volumes

## General

Volume mounts can be defined in the YAML config file.
This can be used to mount files or directories on the host machine (or Persistent Volumes in Kubernetes) into pipeline containers.

!!! important

    Volumes are only available when the "volumes" security options is enabled in the repository settings.
    For security reasons, it should only be used in private environments.

```yaml
steps:
  - name: build
    [...]
    volumes:
      - /etc/ssl/certs:/etc/ssl/certs
```

For the docker backend, volumes are mounted to the host machine.
This requires the use of absolute paths when configuring volumes.
Attempting to use relative paths will result in an error.

## Kubernetes

To mount volumes in Kubernetes, PersistentVolumeClaim (PVC) is which can be referenced in steps via the `volumes` option.

There is no support for dynamic provisioning of PersistentVolumes in Kubernetes.
Persistent volumes, which are referenced from the PVCs, must be created manually/statically.

!!! important

    If the PVC is of type "ReadWriteOnce", you may also need to use affinity and/or nodeSelector settings through [`backend_options`](../configuration/server.md#backend-options_1) to ensure that the steps are executed on the same node the PVC is attached to.

!!! tip

    If a volume should be used in more than one workflow concurrently, make sure to use a "ReadWriteMany" volume.

Assuming a PVC named "cache" exists, it can be referenced as follows:

In a plugin,

```yaml
steps:
  - name: "step name"
    image: <image>
    volumes:
      - cache:/mnt/cache
    [...]
```
