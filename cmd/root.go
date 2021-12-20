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
        Long: `A simple and easy to use remote tunnel server`,
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

    rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default is $HOME/.gogrok.yaml)")
    rootCmd.PersistentFlags().Bool("viper", true, "use Viper for configuration")
    viper.BindPFlag("useViper", rootCmd.PersistentFlags().Lookup("viper"))
}

func initConfig() {
    if cfgFile != "" {
        // Use config file from the flag.
        viper.SetConfigFile(cfgFile)
    } else {
        // Find home directory.
        home, err := os.UserHomeDir()
        cobra.CheckErr(err)

        viper.SetDefault("gogrok.storageDir", path.Join(home, ".gogrok"))

        // Search config in home directory with name ".cobra" (without extension).
        viper.AddConfigPath(home)
        viper.AddConfigPath(path.Join(home, ".gogrok"))
        viper.SetConfigType("yaml")
        viper.SetConfigName(".gogrok")
    }

    viper.AutomaticEnv()

    if err := viper.ReadInConfig(); err == nil {
        fmt.Println("Using config file:", viper.ConfigFileUsed())
    }

    storageDir := viper.GetString("gogrok.storageDir")

    if _, err := os.Stat(storageDir); os.IsNotExist(err) {
        os.MkdirAll(storageDir, 0755)
    }
}