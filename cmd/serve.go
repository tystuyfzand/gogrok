package cmd

import (
	"bufio"
	"fmt"
	"github.com/gliderlabs/ssh"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/afero"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"github.com/spf13/viper"
	"gogrok.ccatss.dev/common"
	"gogrok.ccatss.dev/server"
	gossh "golang.org/x/crypto/ssh"
	"math/rand"
	"path"
	"strings"
)

func init() {
	serveCmd.Flags().String("bind", ":2222", "SSH Server Bind Address")
	serveCmd.Flags().String("http", ":8080", "HTTP Server Bind Address")
	serveCmd.Flags().String("keys", "", "Authorized keys file to control access")
	serveCmd.Flags().StringSlice("domains", nil, "Domains to use for ")
	rootCmd.AddCommand(serveCmd)
}

var serveCmd = &cobra.Command{
	Use:   "serve",
	Short: "Start the gogrok server",
	Run: func(cmd *cobra.Command, args []string) {
		baseFs := afero.NewOsFs()

		viper.SetDefault("gogrok.httpAddress", ":8080")
		viper.SetDefault("gogrok.sshAddress", ":2222")

		setValueFromFlag(cmd.Flags(), "bind", "gogrok.sshAddress", false)
		setValueFromFlag(cmd.Flags(), "http", "gogrok.httpAddress", false)
		setValueFromFlag(cmd.Flags(), "keys", "gogrok.authorizedKeyFile", false)
		setValueFromFlag(cmd.Flags(), "domains", "gogrok.domains", false)

		key, err := common.LoadOrGenerateKey(baseFs, path.Join(viper.GetString("gogrok.storageDir"), "server.key"), "")

		if err != nil {
			log.WithError(err).Fatalln("unable to load or generate server key")
		}

		signer, err := gossh.NewSignerFromKey(key)

		if err != nil {
			log.WithError(err).Fatalln("unable to create signer from key")
		}

		opts := []server.Option{
			server.WithSigner(signer),
		}

		sshServerBind := viper.GetString("gogrok.sshAddress")

		if sshServerBind != "" {
			opts = append(opts, server.WithSSHAddress(sshServerBind))
		}

		if authorizedKeysFile := viper.GetString("gogrok.authorizedKeyFile"); authorizedKeysFile != "" {
			authorizedKeys, err := loadAuthorizedKeys(baseFs, authorizedKeysFile)

			if err != nil {
				log.WithError(err).Fatalln("Unable to load authorized keys file")
				return
			}

			opts = append(opts, server.WithAuthorizedKeys(authorizedKeys))
		}

		if domains := viper.GetStringSlice("gogrok.domains"); domains != nil {
			generator := func() string {
				return server.RandomAnimal() + "." + domains[rand.Intn(len(domains))]
			}

			handler := server.NewHttpHandler(server.WithProvider(generator))

			opts = append(opts, server.WithForwardHandler(handler))
		}

		s, err := server.New(opts...)

		if err != nil {
			log.WithError(err).Fatalln("Unable to start gogrok server")
		}

		httpServerBind := viper.GetString("gogrok.httpAddress")

		log.WithFields(log.Fields{
			"sshAddress":  sshServerBind,
			"httpAddress": httpServerBind,
		}).Info("Starting gogrok server")

		ch := make(chan error)

		go func() {
			ch <- s.Start()
		}()

		go func() {
			ch <- s.StartHTTP(httpServerBind)
		}()

		err = <-ch

		if err != nil {
			log.WithError(err).Fatalln("Unable to start server due to error")
		}
	},
}

// loadAuthorizedKeys loads an authorized keys file
func loadAuthorizedKeys(fs afero.Fs, file string) ([]string, error) {
	f, err := fs.Open(file)

	if err != nil {
		return nil, err
	}

	defer f.Close()

	keys := make([]string, 0)

	s := bufio.NewScanner(f)

	for s.Scan() {
		// Parse and re-serialize our key to support comments/etc
		key, _, _, _, err := ssh.ParseAuthorizedKey(s.Bytes())

		if err != nil {
			continue
		}

		keys = append(keys, strings.TrimSpace(string(gossh.MarshalAuthorizedKey(key))))
	}

	return keys, nil
}

// setValueFromFlag sets a value on the global viper object based on flag key and target key
func setValueFromFlag(flags *pflag.FlagSet, key, targetKey string, force bool) {
	key = strings.TrimSpace(key)
	if (force && flags.Lookup(key) != nil) || flags.Changed(key) {
		f := flags.Lookup(key)
		configKey := key
		if targetKey != "" {
			configKey = targetKey
		}
		// Gotta love this API.
		switch f.Value.Type() {
		case "bool":
			bv, _ := flags.GetBool(key)
			viper.Set(configKey, bv)
		case "string":
			viper.Set(configKey, f.Value.String())
		case "stringSlice":
			bv, _ := flags.GetStringSlice(key)
			viper.Set(configKey, bv)
		case "int":
			iv, _ := flags.GetInt(key)
			viper.Set(configKey, iv)
		default:
			panic(fmt.Sprintf("update switch with %s", f.Value.Type()))
		}

	}
}
