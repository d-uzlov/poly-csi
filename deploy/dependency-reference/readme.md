
# dependency updates

References:
- https://github.com/kubernetes-csi/external-provisioner/
- https://github.com/kubernetes-csi/external-attacher/
- https://github.com/kubernetes-csi/external-snapshotter/
- https://github.com/kubernetes-csi/external-resizer/
- https://github.com/kubernetes-csi/external-health-monitor/
- https://github.com/kubernetes-csi/node-driver-registrar/

```bash

wget https://raw.githubusercontent.com/kubernetes-csi/external-provisioner/v5.3.0/deploy/kubernetes/rbac.yaml -O ./deploy/dependency-reference/rbac-external-provisioner.yaml
wget https://raw.githubusercontent.com/kubernetes-csi/external-attacher/v4.10.0/deploy/kubernetes/rbac.yaml -O ./deploy/dependency-reference/rbac-external-attacher.yaml
wget https://raw.githubusercontent.com/kubernetes-csi/external-snapshotter/v8.3.0/deploy/kubernetes/csi-snapshotter/rbac-csi-snapshotter.yaml -O ./deploy/dependency-reference/rbac-external-snapshotter.yaml
wget https://raw.githubusercontent.com/kubernetes-csi/external-resizer/v1.14.0/deploy/kubernetes/rbac.yaml -O ./deploy/dependency-reference/rbac-external-resizer.yaml
# wget https://raw.githubusercontent.com/kubernetes-csi/external-health-monitor/v0.14.0/deploy/kubernetes/external-health-monitor-controller/rbac.yaml -O ./deploy/rbac-external-resizer.yaml

```

Minimum k8s version:
- `external-provisioner/v5.3.0`: `1.20`
- `external-attacher/v4.10.0`: `1.17`
- `external-snapshotter/v8.3.0`: `1.25`
- `external-resizer/v1.14.0`: `1.16`
- `node-driver-registrar/v2.15.0`: `1.13`
