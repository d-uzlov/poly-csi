package main

import (
	"context"
	"fmt"
	"os/signal"
	"syscall"

	"github.com/spf13/cobra"
	"meoe.io/poly-csi/pkg/nvmetkeeper"
	"meoe.io/poly-csi/pkg/transport"
	"meoe.io/poly-csi/pkg/transport/localexec"
)

var configCmd = &cobra.Command{
	Use:   "config",
	Short: "manage nvmet subsystems",
}

func init() {
	configCmd.AddCommand(configSaveMergeCmd)
	configCmd.AddCommand(configRestoreMergeCmd)
}

var configSaveMergeCmd = &cobra.Command{
	Use:   "save-merge",
	Short: "read current state of nvmet kernel config and save it as files in config folder, create new config files as needed, keep unused files as is",
	Args:  cobra.NoArgs,
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

		err = k.SaveStateToConfig(ctx)
		if err != nil {
			return fmt.Errorf("SaveStateToConfig: %w", err)
		}
		fmt.Println("saved")
		return nil
	},
}

var configRestoreMergeCmd = &cobra.Command{
	Use:   "restore-merge",
	Short: "create nvmet objects as described in config, ignore existing objects not listed in config",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
		defer cancel()

		localExec, err := localexec.New(ctx)
		if err != nil {
			return fmt.Errorf("create local exec: %w", err)
		}
		ctx = transport.MakeContext(ctx, localExec)

		saveState := false
		k := nvmetkeeper.New(configFolder, useSudo, saveState)

		err = k.RestoreConfig(ctx)
		if err != nil {
			return fmt.Errorf("RestoreConfig: %w", err)
		}
		fmt.Println("restored")
		return nil
	},
}
