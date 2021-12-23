package cmd

import (
	"fmt"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"gogrok.ccatss.dev/client"
	"os"
)

func init() {
	unregisterCmd.Flags().String("server", "localhost:2222", "Gogrok Server Address")
	rootCmd.AddCommand(unregisterCmd)
}

var unregisterCmd = &cobra.Command{
	Use:   "unregister",
	Short: "Unregister a host with a gogrok server",
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

		err := c.Unregister(args[0])

		if err != nil {
			fmt.Fprintln(os.Stderr, "Unable to unregister: "+err.Error())
			os.Exit(1)
		}

		cmd.Println("Successfully unregistered host " + args[0])
	},
}
