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

var subsysCmd = &cobra.Command{
	Use:   "subsys",
	Short: "manage nvmet subsystems",
}

func init() {
	subsysCmd.AddCommand(subsysCreateCmd)
	subsysCmd.AddCommand(subsysDeleteCmd)
	subsysCmd.AddCommand(subsysAttachCmd)
	subsysCmd.AddCommand(subsysDetachCmd)
	subsysCmd.AddCommand(subsysAllowHostCmd)
	subsysCmd.AddCommand(subsysDisallowHostCmd)
}

var subsysCreateCmd = &cobra.Command{
	Use:   "create <subsys_json>",
	Short: "Create nvmet subsys with given configuration. If requested subsys already exists, check that existing config matches requested one",
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
		var c nvmet.Subsys
		err = json.Unmarshal([]byte(args[0]), &c)
		if err != nil {
			return fmt.Errorf("parse json: %w", err)
		}
		err = c.Validate()
		if err != nil {
			return fmt.Errorf("input validation: %w", err)
		}
		err = k.CreateSubsystem(ctx, c)
		if err != nil {
			return fmt.Errorf("CreateSubsys: %w", err)
		}
		fmt.Println("created subsys", c.Nqn)
		return nil
	},
}

var subsysDeleteCmd = &cobra.Command{
	Use:   "delete <nqn>",
	Short: "delete nvmet subsys by nqn",
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
		err = k.DeleteSubsys(ctx, name)
		if err != nil {
			return fmt.Errorf("DeleteSubsys: %w", err)
		}
		fmt.Println("deleted subsys", name)
		return nil
	},
}

var subsysAttachCmd = &cobra.Command{
	Use:   "attach nqn port",
	Short: "Attach subsys to given port",
	Args:  cobra.ExactArgs(2),
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
		port := args[1]

		err = k.AttachNqnToPort(ctx, name, port)
		if err != nil {
			return fmt.Errorf("AttachNqnToPort: %w", err)
		}
		fmt.Println("attached subsys", name, "to", port)
		return nil
	},
}

var subsysDetachCmd = &cobra.Command{
	Use:   "detach nqn port",
	Short: "Detach subsys from given port",
	Args:  cobra.ExactArgs(2),
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
		port := args[1]

		err = k.DetachPortFromNqn(ctx, name, port)
		if err != nil {
			return fmt.Errorf("DetachPortFromNqn: %w", err)
		}
		fmt.Println("detached subsys", name, "from", port)
		return nil
	},
}

var subsysAllowHostCmd = &cobra.Command{
	Use:   "allow nqn host_nqn",
	Short: "Allow a host to access subsystem",
	Args:  cobra.ExactArgs(2),
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
		hostNqn := args[1]

		err = k.AllowHost(ctx, name, hostNqn)
		if err != nil {
			return fmt.Errorf("AllowHost: %w", err)
		}
		fmt.Println("allow subsys", name, "to be accessed by", hostNqn)
		return nil
	},
}

var subsysDisallowHostCmd = &cobra.Command{
	Use:   "disallow nqn host_nqn",
	Short: "Disallow a host to access subsystem",
	Args:  cobra.ExactArgs(2),
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
		hostNqn := args[1]

		err = k.DisallowHost(ctx, name, hostNqn)
		if err != nil {
			return fmt.Errorf("DisallowHost: %w", err)
		}
		fmt.Println("disallow subsys", name, "to be accessed by", hostNqn)
		return nil
	},
}
