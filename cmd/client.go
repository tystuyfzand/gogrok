package cmd

import (
	"errors"
	"fmt"
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
	clientCmd.Flags().String("host", "", "Requested host to register")
	rootCmd.AddCommand(clientCmd)
}

func clientPreRun(cmd *cobra.Command, args []string) {
	viper.SetDefault("gogrok.server", "localhost:2222")

	setValueFromFlag(cmd.Flags(), "server", "gogrok.server", false)
	setValueFromFlag(cmd.Flags(), "key", "gogrok.clientKey", false)
	setValueFromFlag(cmd.Flags(), "passphrase", "gogrok.clientKeyPassphrase", false)
}

func loadClientKey() ssh.Signer {
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

	return signer
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
	PreRun: clientPreRun,
	Run: func(cmd *cobra.Command, args []string) {
		setValueFromFlag(cmd.Flags(), "host", "gogrok.clientHost", false)

		c := client.New(viper.GetString("gogrok.server"), loadClientKey())

		host, err := c.Start(args[0], viper.GetString("gogrok.clientHost"))

		if err != nil {
			fmt.Fprintln(os.Stderr, "Unable to start server: "+err.Error())
			os.Exit(1)
		}

		cmd.Println("Successfully bound host and started proxy")
		log.WithField("host", host).Info("Successfully bound host and started proxy")

		cmd.Println("Endpoints:")
		cmd.Printf("http://%s\n", host)
		cmd.Printf("https://%s\n", host)

		sig := make(chan os.Signal)

		signal.Notify(sig, syscall.SIGTERM, syscall.SIGINT, syscall.SIGKILL)

		<-sig
	},
}
