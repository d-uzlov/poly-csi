package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var (
	configFolder string
	useSudo      bool
)

func main() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

var rootCmd = &cobra.Command{
	Use:           "nvmet_keeper",
	Short:         "nvmet_keeper CLI",
	Long:          "",
	SilenceUsage:  true,
	SilenceErrors: true,
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		return nil
	},
}

func init() {
	// flags that should be available to all subcommands
	rootCmd.PersistentFlags().StringVar(&configFolder, "config-folder", "/etc/nvmet_keeper", "path to config folder")
	// users must invoke nvmet_keeper using sudo instead of requesting per-operation sudo
	// rootCmd.PersistentFlags().BoolVar(&useSudo, "use-sudo", false, "run commands with sudo")

	rootCmd.AddCommand(portCmd)
	rootCmd.AddCommand(subsysCmd)
	rootCmd.AddCommand(namespaceCmd)
	rootCmd.AddCommand(configCmd)
	rootCmd.AddCommand(hostCmd)
	rootCmd.AddCommand(stateCmd)
	rootCmd.AddCommand(compareCmd)
}
