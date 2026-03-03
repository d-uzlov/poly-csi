package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os/signal"
	"syscall"

	"github.com/spf13/cobra"
	"meoe.io/poly-csi/pkg/nvmetkeeper"
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
	Example: `
nvmet_keeper subsys create '{"nqn":"nqn.2014-08.org.com.example:subsys","model":"","serial":"","allow_any_host":false}'
model and serial can be arbitrary strings. When left empty, Linux kernel will set model=Linux and generate a unique value for serial
`,
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
		defer cancel()

		localExec, err := localexec.New()
		if err != nil {
			return fmt.Errorf("create local exec: %w", err)
		}

		saveState := true
		k := nvmetkeeper.New(localExec, configFolder, useSudo, saveState)
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
	Example: `nvmet_keeper subsys delete nqn.2014-08.org.com.example:subsys`,
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
		defer cancel()

		localExec, err := localexec.New()
		if err != nil {
			return fmt.Errorf("create local exec: %w", err)
		}

		saveState := true
		k := nvmetkeeper.New(localExec, configFolder, useSudo, saveState)
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
	Use:   "attach nqn port_name",
	Short: "Attach subsys to given port",
	Example: `nvmet_keeper subsys attach nqn.2014-08.org.com.example:subsys 2020`,
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
		defer cancel()

		localExec, err := localexec.New()
		if err != nil {
			return fmt.Errorf("create local exec: %w", err)
		}

		saveState := true
		k := nvmetkeeper.New(localExec, configFolder, useSudo, saveState)

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
	Use:   "detach nqn port_name",
	Short: "Detach subsys from given port",
	Example: `nvmet_keeper subsys detach nqn.2014-08.org.com.example:subsys 2020`,
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
		defer cancel()

		localExec, err := localexec.New()
		if err != nil {
			return fmt.Errorf("create local exec: %w", err)
		}

		saveState := true
		k := nvmetkeeper.New(localExec, configFolder, useSudo, saveState)

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
	Example: `nvmet_keeper subsys allow nqn.2014-08.org.com.example:subsys nqn.2014-08.org.com.example:machine`,
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
		defer cancel()

		localExec, err := localexec.New()
		if err != nil {
			return fmt.Errorf("create local exec: %w", err)
		}

		saveState := true
		k := nvmetkeeper.New(localExec, configFolder, useSudo, saveState)

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
	Example: `nvmet_keeper subsys disallow nqn.2014-08.org.com.example:subsys nqn.2014-08.org.com.example:machine`,
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
		defer cancel()

		localExec, err := localexec.New()
		if err != nil {
			return fmt.Errorf("create local exec: %w", err)
		}

		saveState := true
		k := nvmetkeeper.New(localExec, configFolder, useSudo, saveState)

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
