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
}

func init() {
	hostCmd.AddCommand(hostCreateCmd)
}

var hostCreateCmd = &cobra.Command{
	Use:   "create <host_json>",
	Short: "Create nvmet host with given configuration. If requested host already exists, check that existing config matches requested one",
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
