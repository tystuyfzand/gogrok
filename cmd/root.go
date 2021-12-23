package cmd

import (
	"fmt"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"os"
	"path"
)

var (
	cfgFile string
	rootCmd = &cobra.Command{
		Use:   "gogrok",
		Short: "Gogrok is a very simple and easy to use reverse tunnel server",
		Long:  `A simple and easy to use remote tunnel server`,
		Run: func(cmd *cobra.Command, args []string) {
			clientCmd.Run(cmd, args)
		},
	}
)

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func init() {
	cobra.OnInitialize(initConfig)

	rootCmd.Flags().String("key", "", "Server/Client key file")
	rootCmd.Flags().String("passphrase", "", "Server/Client key passphrase")
	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default is $HOME/.gogrok.yaml)")
	rootCmd.PersistentFlags().Bool("viper", true, "use Viper for configuration")
	viper.BindPFlag("useViper", rootCmd.PersistentFlags().Lookup("viper"))
}

func initConfig() {
	home, err := os.UserHomeDir()
	cobra.CheckErr(err)

	if cfgFile != "" {
		// Use config file from the flag.
		viper.SetConfigFile(cfgFile)
	} else {
		// Find home directory.

		// Search config in home directory with name ".cobra" (without extension).
		viper.AddConfigPath(home)
		viper.AddConfigPath(path.Join(home, ".gogrok"))
		viper.SetConfigType("yaml")
		viper.SetConfigName(".gogrok")
	}

	viper.SetDefault("gogrok.storageDir", path.Join(home, ".gogrok"))

	// Generic binds
	viper.BindEnv("gogrok.storageDir", "GOGROK_STORAGE_DIR")

	// Server binds
	viper.BindEnv("gogrok.sshAddress", "GOGROK_SSH_ADDRESS")
	viper.BindEnv("gogrok.httpAddress", "GOGROK_HTTP_ADDRESS")
	viper.BindEnv("gogrok.authorizedKeyFile", "GOGROK_AUTHORIZED_KEY_FILE")
	viper.BindEnv("gogrok.domains", "GOGROK_DOMAINS")

	// Client binds
	viper.BindEnv("gogrok.clientKey", "GOGROK_CLIENT_KEY")
	viper.BindEnv("gogrok.clientKeyPassphrase", "GOGROK_CLIENT_KEY_PASS")
	viper.BindEnv("gogrok.server", "GOGROK_SERVER")

	viper.AutomaticEnv()

	if err := viper.ReadInConfig(); err == nil {
		fmt.Println("Using config file:", viper.ConfigFileUsed())
	}

	storageDir := viper.GetString("gogrok.storageDir")

	if _, err := os.Stat(storageDir); os.IsNotExist(err) {
		os.MkdirAll(storageDir, 0755)
	}
}
