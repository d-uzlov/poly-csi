package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/spf13/cobra"
	"meoe.io/poly-csi/pkg/nvmetkeeper"
	"meoe.io/poly-csi/pkg/transport"
	"meoe.io/poly-csi/pkg/transport/localexec"
)

var stateCmd = &cobra.Command{
	Use:   "state",
	Short: "manage nvmet state",
}

func init() {
	stateCmd.AddCommand(stateShowCmd)
	stateCmd.AddCommand(stateResetAllCmd)
}

var stateShowCmd = &cobra.Command{
	Use:   "show",
	Short: "Create nvmet state with given configuration. If requested state already exists, check that existing config matches requested one",
	Args:  cobra.ExactArgs(0),
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
		defer cancel()

		localExec, err := localexec.New(ctx)
		if err != nil {
			return fmt.Errorf("create local exec: %w", err)
		}
		ctx = transport.MakeContext(ctx, localExec)

		// var b bytes.Buffer
		// w := bufio.NewWriter(&b)

		saveState := true
		k := nvmetkeeper.New(configFolder, useSudo, saveState)
		err = k.ShowState(ctx, os.Stdout)
		if err != nil {
			return fmt.Errorf("ShowState: %w", err)
		}
		// fmt.Print(b.String())
		return nil
	},
}

var stateResetAllCmd = &cobra.Command{
	Use:   "reset-all",
	Short: "Reset all nvmet state. Does not touch config on disk",
	Args:  cobra.ExactArgs(0),
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
		err = k.ResetState(ctx)
		if err != nil {
			return fmt.Errorf("ResetState: %w", err)
		}
		fmt.Println("reset successful")
		return nil
	},
}
