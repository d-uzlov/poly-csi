package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/spf13/cobra"
	"meoe.io/poly-csi/pkg/nvmetkeeper"
	"meoe.io/poly-csi/pkg/transport/localexec"
)

var compareCmd = &cobra.Command{
	Use:   "compare",
	Short: "bulk-manage in-memory state of nvmet",
}

func init() {
	compareCmd.AddCommand(compareHostCmd)
	compareCmd.AddCommand(comparePortCmd)
	compareCmd.AddCommand(compareSubsysCmd)
	compareCmd.AddCommand(compareAllCmd)
}

var compareHostCmd = &cobra.Command{
	Use:     "host",
	Aliases: []string{"hosts"},
	Short:   "Compare current nvmet state with values saved in config",
	Args:    cobra.ExactArgs(0),
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
		defer cancel()

		localExec := localexec.New(localexec.Config{})

		// var b bytes.Buffer
		// w := bufio.NewWriter(&b)

		saveState := true
		k := nvmetkeeper.New(localExec, configFolder, useSudo, saveState)
		err := k.CompareHostState(ctx, os.Stdout)
		if err != nil {
			return fmt.Errorf("CompareHostState: %w", err)
		}
		// fmt.Print(b.String())
		return nil
	},
}

var comparePortCmd = &cobra.Command{
	Use:     "port",
	Aliases: []string{"ports"},
	Short:   "Compare current nvmet state with values saved in config",
	Args:    cobra.ExactArgs(0),
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
		defer cancel()

		localExec := localexec.New(localexec.Config{})

		// var b bytes.Buffer
		// w := bufio.NewWriter(&b)

		saveState := true
		k := nvmetkeeper.New(localExec, configFolder, useSudo, saveState)
		err := k.ComparePortState(ctx, os.Stdout)
		if err != nil {
			return fmt.Errorf("ComparePortState: %w", err)
		}
		// fmt.Print(b.String())
		return nil
	},
}

var compareSubsysCmd = &cobra.Command{
	Use:     "subsys",
	Aliases: []string{"subsystems"},
	Short:   "Compare current nvmet state with values saved in config",
	Args:    cobra.ExactArgs(0),
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
		defer cancel()

		localExec := localexec.New(localexec.Config{})

		// var b bytes.Buffer
		// w := bufio.NewWriter(&b)

		saveState := true
		k := nvmetkeeper.New(localExec, configFolder, useSudo, saveState)
		err := k.CompareSubsysState(ctx, os.Stdout)
		if err != nil {
			return fmt.Errorf("CompareSubsysState: %w", err)
		}
		// fmt.Print(b.String())
		return nil
	},
}

var compareAllCmd = &cobra.Command{
	Use:   "all",
	Short: "Compare current nvmet state with values saved in config",
	Args:  cobra.ExactArgs(0),
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
		defer cancel()

		localExec := localexec.New(localexec.Config{})

		// var b bytes.Buffer
		// w := bufio.NewWriter(&b)

		saveState := true
		k := nvmetkeeper.New(localExec, configFolder, useSudo, saveState)
		err := k.ComparePortState(ctx, os.Stdout)
		if err != nil {
			return fmt.Errorf("ComparePortState: %w", err)
		}
		err = k.CompareHostState(ctx, os.Stdout)
		if err != nil {
			return fmt.Errorf("CompareHostState: %w", err)
		}
		err = k.CompareSubsysState(ctx, os.Stdout)
		if err != nil {
			return fmt.Errorf("CompareSubsysState: %w", err)
		}
		// fmt.Print(b.String())
		return nil
	},
}
