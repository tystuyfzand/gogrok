package cmd

import (
	"errors"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/afero"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"gogrok.ccatss.dev/client"
	"gogrok.ccatss.dev/common"
	"golang.org/x/crypto/ssh"
	"os"
	"os/signal"
	"path"
	"syscall"
)

var (
	ErrNoEndpoint = errors.New("no http(s) endpoint provided")
)

func init() {
	clientCmd.Flags().String("server", "localhost:2222", "Gogrok Server Address")
	clientCmd.Flags().String("key", "", "Client key file")
	clientCmd.Flags().String("passphrase", "", "Client key passphrase")
	rootCmd.AddCommand(clientCmd)
}

var clientCmd = &cobra.Command{
	Use:   "client",
	Short: "Start the gogrok client",
	Args: func(cmd *cobra.Command, args []string) error {
		if len(args) < 1 {
			return ErrNoEndpoint
		}
		return nil
	},
	Run: func(cmd *cobra.Command, args []string) {
		viper.SetDefault("gogrok.server", "localhost:2222")

		setValueFromFlag(cmd.Flags(), "server", "gogrok.server", false)
		setValueFromFlag(cmd.Flags(), "key", "gogrok.clientKey", false)
		setValueFromFlag(cmd.Flags(), "passphrase", "gogrok.clientKeyPassphrase", false)

		clientKey := viper.GetString("gogrok.clientKey")

		if clientKey == "" {
			clientKey = path.Join(viper.GetString("gogrok.storageDir"), "client.key")
		}

		key, err := common.LoadOrGenerateKey(afero.NewOsFs(), clientKey, viper.GetString("gogrok.clientKeyPassphrase"))

		if err != nil {
			log.WithError(err).Fatalln("Unable to load client key")
		}

		signer, err := ssh.NewSignerFromKey(key)

		if err != nil {
			log.WithError(err).Fatalln("Unable to create signer from client key")
		}

		// Default command is client
		c := client.New(viper.GetString("gogrok.server"), args[0], signer)

		err = c.Start()

		if err != nil {
			log.WithError(err).Fatalln("Unable to connect to server")
		}

		sig := make(chan os.Signal)

		signal.Notify(sig, syscall.SIGTERM, syscall.SIGINT, syscall.SIGKILL)

		<-sig
	},
}
