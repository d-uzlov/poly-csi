
# /api

`/api` folder contains public JSON serialization definitions for various RPCs used by poly-csi.

## generate

```bash

(cd ~ && npm install quicktype)
# add ./node_modules/.bin to your path

quicktype --src ./api/node_registry/node-registry.ts -o ./api/node_registry/node-registry.gen.go --package node_registry --just-types-and-package --omit-empty
quicktype --src ./api/node_registry/node-registry.ts -o ./api/node_registry/node-registry.gen.rs

```
