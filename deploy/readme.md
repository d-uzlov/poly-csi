
# deploy

```bash

kl create ns pv-poly

kl apply -f ./deploy/controller/rbac.yaml

kl -n pv-poly create configmap --dry-run=client -o yaml controller-config             --from-file config.yaml=./config-controller.yaml | kl apply -f -
kl -n pv-poly create configmap --dry-run=client -o yaml node-config                   --from-file config.yaml=./deploy/node/config-node.yaml | kl apply -f -
kl -n pv-poly create configmap --dry-run=client -o yaml node-plugin-iscsi-config      --from-file config.yaml=./deploy/node/config-iscsi.yaml | kl apply -f -
kl -n pv-poly create configmap --dry-run=client -o yaml node-plugin-multipath-config  --from-file config.yaml=./deploy/node/config-multipath.yaml | kl apply -f -

ssh_key=~/.ssh/id_ed25519
kl -n pv-poly create secret --dry-run=client -o yaml generic ssh-keys               --from-file ssh_key_1=$ssh_key | kl apply -f -
kl -n pv-poly create secret --dry-run=client -o yaml generic controller-public-key  --from-file controller_key.pub.pem=./.env/controller_key.pub.pem | kl apply -f -
kl -n pv-poly create secret --dry-run=client -o yaml generic controller-private-key --from-file controller_key.pem=./.env/controller_key.pem | kl apply -f -

kl apply -f ./deploy/node-registry/node-registry.yaml
kl -n pv-poly get pod

# you need to add "uni_csi_nodes" group to your ansible inventory
ansible-inventory --graph uni_csi_nodes
ansible-playbook ./deploy/plugins/iscsi-playbook.yaml

kl apply -f ./deploy/node/node-plugins.yaml
kl apply -f ./deploy/node/daemonset.yaml
kl apply -f ./deploy/controller/deployment.yaml
kl -n pv-poly get pod
kl -n pv-poly get ds
kl -n pv-poly get deploy

kl -n pv-poly logs deployments/poly-csi-controller --tail 10
kl -n pv-poly logs deployments/poly-csi-controller > ./controller.log
kl -n pv-poly logs deployments/poly-csi-controller > ./controller.log --follow

```

# cleanup

```bash

kl delete -f ./deploy/controller/deployment.yaml
kl delete -f ./deploy/controller/rbac.yaml
kl delete -f ./deploy/node/daemonset.yaml
kl -n pv-poly delete cm main-config
kl -n pv-poly delete secret ssh-keys

```

# test nfs

```bash

kl apply -f ./deploy/test/test-nfs-storage-class.yaml

kl apply -f ./deploy/test/test-nfs-pvc.yaml
kl describe pvc test-sc-poly-csi-nfs
pv_name=$(kl get pvc test-sc-poly-csi-nfs -o json | jq .spec.volumeName -r)
kl describe pv $pv_name

kl apply -f ./deploy/test/test-nfs-deployment.yaml
kl get pod -l app=test-sc-poly-csi-nfs
kl describe pod -l app=test-sc-poly-csi-nfs

pod_node=$(kl get pod -l app=test-sc-poly-csi-nfs -o json | jq .items[0].spec.nodeName -r)
ds_pod=$(kl -n pv-poly get pod -l app.kubernetes.io/csi-role=node --field-selector spec.nodeName=$pod_node -o name)
kl -n pv-poly logs $ds_pod

volume_attachment=$(kl get volumeattachment -o json | jq ".items.[] | select((.spec.nodeName == \"$pod_node\") and (.spec.source.persistentVolumeName == \"$pv_name\")) | .metadata.name" -r)
kl describe volumeattachment $volume_attachment

kl exec deployments/test-sc-poly-csi-nfs -it -- df -h
kl exec deployments/test-sc-poly-csi-nfs -it -- sh

kl delete -f ./deploy/test/test-nfs-deployment.yaml
kl delete -f ./deploy/test/test-nfs-pvc.yaml
kl delete -f ./deploy/test/test-nfs-storage-class.yaml

```

# test nvme

```bash

kl apply -f ./deploy/test/test-nvme-storage-class.yaml
kl describe sc poly-csi-nvme

kl apply -f ./deploy/test/test-nvme-pvc.yaml
kl describe pvc test-sc-poly-csi-nvme
pv_name=$(kl get pvc test-sc-poly-csi-nvme -o json | jq .spec.volumeName -r)
kl describe pv $pv_name

kl apply -f ./deploy/test/test-nvme-deployment.yaml
kl get pod -l app=test-sc-poly-csi-nvme
kl describe pod -l app=test-sc-poly-csi-nvme

pod_node=$(kl get pod -l app=test-sc-poly-csi-nvme -o json | jq .items[0].spec.nodeName -r)
ds_pod=$(kl -n pv-poly get pod -l app.kubernetes.io/csi-role=node --field-selector spec.nodeName=$pod_node -o name)
kl -n pv-poly logs $ds_pod

kl exec deployments/test-sc-poly-csi-nvme -it -- df -h
kl exec deployments/test-sc-poly-csi-nvme -it -- sh

kl delete -f ./deploy/test/test-nvme-deployment.yaml
kl delete -f ./deploy/test/test-nvme-pvc.yaml
kl delete -f ./deploy/test/test-nvme-storage-class.yaml

```

# test iscsi

```bash

kl apply -f ./deploy/test/test-iscsi-storage-class.yaml
kl describe sc poly-csi-iscsi

kl apply -f ./deploy/test/test-iscsi-pvc.yaml
kl describe pvc test-sc-poly-csi-iscsi
pv_name=$(kl get pvc test-sc-poly-csi-iscsi -o json | jq .spec.volumeName -r)
kl describe pv $pv_name

kl apply -f ./deploy/test/test-iscsi-deployment.yaml
kl get pod -l app=test-sc-poly-csi-iscsi
kl describe pod -l app=test-sc-poly-csi-iscsi

pod_node=$(kl get pod -l app=test-sc-poly-csi-iscsi -o json | jq .items[0].spec.nodeName -r)
pod_node=trixie-work-4
ds_pod=$(kl -n pv-poly get pod -l app.kubernetes.io/csi-role=node --field-selector spec.nodeName=$pod_node -o name)
kl -n pv-poly logs $ds_pod

kl exec deployments/test-sc-poly-csi-iscsi -it -- df -h
kl exec deployments/test-sc-poly-csi-iscsi -it -- sh

kl delete -f ./deploy/test/test-iscsi-deployment.yaml
kl delete -f ./deploy/test/test-iscsi-pvc.yaml
kl delete -f ./deploy/test/test-iscsi-storage-class.yaml

```
