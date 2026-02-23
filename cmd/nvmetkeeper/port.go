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

var portCmd = &cobra.Command{
	Use:   "port",
	Short: "manage nvmet ports",
}

func init() {
	portCmd.AddCommand(portCreateCmd)
	portCmd.AddCommand(portDeleteCmd)
}

var portCreateCmd = &cobra.Command{
	Use:   "create <port_json>",
	Short: "Create nvmet port with given configuration. If requested port already exists, check that existing config matches requested one",
	Args:  cobra.ExactArgs(1),
	Example: `
nvmet_keeper port create '{"name":"2020","address_family":"ipv4","listen_addr":"0.0.0.0","listen_port":2020,"type":"tcp"}'

While name and listen_port are logically different values: listen_port must be a number for a TCP port, and name is just a string, which is used only locally, for configuration.
However, name must also be a number, at least in the current implementation in Linux.
Since you can't give ports readable names, the most reasonable usage is to make name=listen_port.

type can theoretically be any NVMe transport type value supported by Linux, but only tcp is tested.
`,
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
		var c nvmet.Port
		err = json.Unmarshal([]byte(args[0]), &c)
		if err != nil {
			return fmt.Errorf("parse json: %w", err)
		}
		err = c.Validate()
		if err != nil {
			return fmt.Errorf("input validation: %w", err)
		}
		err = k.CreatePort(ctx, c)
		if err != nil {
			return fmt.Errorf("CreatePort: %w", err)
		}
		fmt.Println("created port", c.Name)
		return nil
	},
}

var portDeleteCmd = &cobra.Command{
	Use:   "delete <port_name>",
	Short: "delete nvmet port by name",
	Example: `nvmet_keeper port delete 2020`,
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
		name := args[0]
		err = k.DeletePort(ctx, name)
		if err != nil {
			return fmt.Errorf("DeletePort: %w", err)
		}
		fmt.Println("deleted port", name)
		return nil
	},
}
