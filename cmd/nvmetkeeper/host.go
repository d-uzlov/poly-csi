package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os/signal"
	"syscall"

	"github.com/spf13/cobra"
	"meoe.io/poly-csi/pkg/nvmetkeeper"
	"meoe.io/poly-csi/pkg/transport"
	"meoe.io/poly-csi/pkg/transport/localexec"
	"meoe.io/poly-csi/pkg/utils/nvmet"
)

var hostCmd = &cobra.Command{
	Use:   "host",
	Short: "manage nvmet hosts",
	Long: `Hosts are expected to be immutable most of the time, so there are currently no commands to alter host state or delete them.
Separate commands for host state update and deletion may be added in the future.`,
}

func init() {
	hostCmd.AddCommand(hostCreateCmd)
}

var hostCreateCmd = &cobra.Command{
	Use:   "create <host_json>",
	Short: "Create nvmet host with given configuration. If requested host already exists, check that existing config matches requested one",
	Example: `
nvmet_keeper host create '{"nqn":"nqn.2014-08.org.com.example:machine","need_auth":false,"host_key":"","controller_key":"","hash_type":"","dh_group":""}'
When need_auth=true, all host_key, controller_key, hash_type, dh_group must be specified.

- host_key and controller_key must be generated using 'nvme gen-dhchap-key'.
- hash_type must be a value from the following list (values are not validated, this is just a list of values supported on Linux 6.12):
- - hmac(sha256)
- - hmac(sha384)
- - hmac(sha512)
- dh_group must be a value from the following list (values are not validated, this is just a list of values supported on Linux 6.12):
- - 0
- - ffdhe2048
- - ffdhe3072
- - ffdhe4096
- - ffdhe6144
- - ffdhe8192
`,
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
		defer cancel()

		localExec, err := localexec.New(ctx)
		if err != nil {
			return fmt.Errorf("create local exec: %w", err)
		}
		ctx = transport.MakeContext(ctx, localExec)

		saveState := true
		k := nvmetkeeper.New(configFolder, useSudo, saveState)
		var c nvmet.Host
		err = json.Unmarshal([]byte(args[0]), &c)
		if err != nil {
			return fmt.Errorf("parse json: %w", err)
		}
		err = c.Validate()
		if err != nil {
			return fmt.Errorf("input validation: %w", err)
		}
		err = k.CreateHost(ctx, c)
		if err != nil {
			return fmt.Errorf("CreatePort: %w", err)
		}
		fmt.Println("created host", c.Nqn)
		return nil
	},
}
