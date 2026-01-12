
# Controller config

This file describes format of the controller-deployment config.

## Full example

```yaml
nodeInfoDb:
  address: http://poly-csi-node-registry:8080

encrypt:
  controllerKey:
    path: /poly-csi/controller-private-key/controller_key.pem

servers:
- name: rose
  stableId: 197bf7b716
  execType: ssh
  execConfig:
    serverAddress: nas-rose.example.com:22
    username: user
    defaultCommandTimeout: 1s
    insecureEchoCommands: false
    auth:
    - name: k8s-secret
      type: key
      valueFromFile: /poly-csi/ssh-keys/ssh_key_1
  metadataType: json-file
  metadataConfig:
    directory: /etc/poly-csi/volumes/
  addressList:
  - name: storage
    stableId: 22fb9b60
    address: nas-rose.storage.example.com
  - name: k8s
    stableId: 6d7d4d62
    address: nas-rose.example.com
  commands:
    nvmet_keeper: [ sudo, /opt/nvmet_keeper, --config-folder=/etc/nvmet_keeper ]
  globalParams:
    nqnBase: nqn.2025-01.com.example:rose
    iqnBase: iqn.2025-01.com.example:rose

- name: truenas
  stableId: 4149bf2215
  execType: ssh
  execConfig:
    serverAddress: truenas.example.com:22
    username: user
    defaultCommandTimeout: 1s
    insecureEchoCommands: false
    auth:
    - name: k8s-secret
      type: key
      valueFromFile: /poly-csi/ssh-keys/ssh_key_1
  metadataType: json-file
  metadataConfig:
    directory: /data/poly-csi/volumes/
  addressList:
  - name: k8s
    stableId: 6d7d4d62
    address: truenas.example.com
  globalParams:
    nqnBase: nqn.2025-01.com.example:truenas
    iqnBase: nqn.2025-01.com.example:truenas

```

## Servers

Main object in the controller is the list of storage servers, defines in the `servers` config key.

Each server object is supposed to describe separate storage server.
Server config describes current server state and can be changed at any time.

Any server management operations will use current server config, even for existing volume created with different config.
This also means that if server config is broken or deleted, existing volumes become inoperable.

For details about server settings see [server config description](./controller-server-config.md).

## Node metadata storage

Controller must have access to node metadata storage, as described in the [design overview](./design.md#extra-components-node-metadata)

Full example:

```yaml
nodeInfoDb:
  address: http://poly-csi-node-registry:8080
```

`address` parameter specifies how to connect to node metadata server.

Schema prefix is required (`http://` or `https://`).

Currently authentication is TODO.

## Encryption

CSI spec does not have a way to pass encrypted data from node to controller deployment,
so we need to encrypt data manually.

Nodes must have access to controller public key.

`encrypt` config must point to controller private key, so that controller can decrypt data from nodes.

Full example:

```yaml
encrypt:
  controllerKey:
    path: /poly-csi/controller-private-key/controller_key.pem
```

TODO remove this?
It doesn't seem like nodes even need secret metadata at all.
