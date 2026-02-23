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

var namespaceCmd = &cobra.Command{
	Use:   "namespace",
	Short: "manage nvmet namespaces",
	Long: `Namespaces are expected to be immutable most of the time, so there are currently no commands to alter namespace state.
Namespace deletion is handled as part of subsys deletion.
Separate functions for updating namespace configuration and namespace deletion may be added in the future.`,
}

func init() {
	namespaceCmd.AddCommand(namespaceCreateCmd)
}

var namespaceCreateCmd = &cobra.Command{
	Use:   "create <namespace_json>",
	Short: "Create nvmet namespace with given configuration. If requested namespace already exists, check that existing config matches requested one",
	Example: `
nvmet_keeper namespace create '{"nqn":"nqn.2014-08.org.com.example:subsys","nsid":"1","device_path":"/dev/null","device_uuid":"dbf6502b-6dc5-49ec-b983-791c8e05311b","device_guid":"dbf6502b-6dc5-49ec-b983-791c8e05311b","enabled":true}'

- nqn is NQN of parent subsys
- nsid is namespace index, starting from 1
- device_uuid and device_guid are optional. By default Linux will generate unique UUID values for them.
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
		var c nvmet.Ns
		err = json.Unmarshal([]byte(args[0]), &c)
		if err != nil {
			return fmt.Errorf("parse json: %w", err)
		}
		err = c.Validate()
		if err != nil {
			return fmt.Errorf("input validation: %w", err)
		}
		err = k.CreateNamespace(ctx, c)
		if err != nil {
			return fmt.Errorf("CreateNamespace: %w", err)
		}
		fmt.Println("created namespace", c.Nqn)
		return nil
	},
}
