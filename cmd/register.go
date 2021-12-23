package cmd

import (
	"errors"
	"fmt"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"gogrok.ccatss.dev/client"
	"os"
)

var (
	ErrNoHost = errors.New("no host specified")
)

func init() {
	registerCmd.Flags().String("server", "localhost:2222", "Gogrok Server Address")
	rootCmd.AddCommand(registerCmd)
}

var registerCmd = &cobra.Command{
	Use:   "register",
	Short: "Register a host with a gogrok server",
	Args: func(cmd *cobra.Command, args []string) error {
		if len(args) < 1 {
			return ErrNoHost
		}
		return nil
	},
	PreRun: clientPreRun,
	Run: func(cmd *cobra.Command, args []string) {
		// Default command is client
		c := client.New(viper.GetString("gogrok.server"), loadClientKey())

		err := c.Register(args[0])

		if err != nil {
			fmt.Fprintln(os.Stderr, "Unable to register: "+err.Error())
			os.Exit(1)
		}

		cmd.Println("Successfully registered host " + args[0])
	},
}
