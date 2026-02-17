
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
