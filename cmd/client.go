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
	serverHost          string
	clientKey           string
	clientKeyPassphrase string

	ErrNoEndpoint = errors.New("no http(s) endpoint provided")
)

func init() {
	clientCmd.PersistentFlags().StringVar(&serverHost, "server", "localhost:2222", "Gogrok Server Address")
	clientCmd.PersistentFlags().StringVar(&clientKey, "key", "", "Client key file")
	clientCmd.PersistentFlags().StringVar(&clientKeyPassphrase, "passphrase", "", "Client key passphrase")
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
		if clientKey == "" {
			clientKey = path.Join(viper.GetString("gogrok.storageDir"), "client.key")
		}

		key, err := common.LoadOrGenerateKey(afero.NewOsFs(), clientKey, clientKeyPassphrase)

		if err != nil {
			log.WithError(err).Fatalln("Unable to load client key")
		}

		signer, err := ssh.NewSignerFromKey(key)

		if err != nil {
			log.WithError(err).Fatalln("Unable to create signer from client key")
		}

		// Default command is client
		c := client.New(serverHost, args[0], signer)

		err = c.Start()

		if err != nil {
			log.WithError(err).Fatalln("Unable to connect to server")
		}

		sig := make(chan os.Signal)

		signal.Notify(sig, syscall.SIGTERM, syscall.SIGINT, syscall.SIGKILL)

		<-sig
	},
}
