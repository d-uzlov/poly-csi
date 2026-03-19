
# deploy

```bash

kl create ns pv-poly

kl apply -f ./deploy/controller/rbac.yaml

kl -n pv-poly create configmap --dry-run=client -o yaml controller-config             --from-file config.yaml=./config-controller.yaml | kl apply -f -
kl -n pv-poly create configmap --dry-run=client -o yaml node-config                   --from-file config.yaml=./deploy/node/config-node.yaml | kl apply -f -
kl -n pv-poly create configmap --dry-run=client -o yaml node-plugin-iscsi-config      --from-file config.yaml=./deploy/node/config-iscsi.yaml | kl apply -f -

ssh_key=~/.ssh/id_ed25519
kl -n pv-poly create secret --dry-run=client -o yaml generic ssh-keys               --from-file ssh_key_1=$ssh_key | kl apply -f -

kl apply -f ./deploy/node-registry/node-registry.yaml
kl -n pv-poly get pod

kl apply -f ./deploy/node/daemonset.yaml
kl apply -f ./deploy/controller/deployment.yaml
kl -n pv-poly get pod
kl -n pv-poly get ds
kl -n pv-poly get deploy

kl -n pv-poly logs deployments/poly-csi-controller --tail 10
kl -n pv-poly logs deployments/poly-csi-controller csi-driver > ./controller.log
kl -n pv-poly logs deployments/poly-csi-controller csi-driver > ./controller.log --follow

```

# cleanup

```bash

kl delete -f ./deploy/controller/deployment.yaml
kl delete -f ./deploy/controller/rbac.yaml
kl delete -f ./deploy/node/daemonset.yaml
kl delete -f ./deploy/node-registry/node-registry.yaml
kl -n pv-poly delete cm main-config
kl -n pv-poly delete secret ssh-keys

```
