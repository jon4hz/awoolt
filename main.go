package main

import (
	"fmt"
	"os"
	"path"
	"strings"

	"github.com/adrg/xdg"
	"github.com/charmbracelet/huh"
	"github.com/charmbracelet/huh/spinner"
	"github.com/charmbracelet/log"
	"github.com/charmbracelet/x/term"
	"github.com/jon4hz/awoolt/config"
	"github.com/jon4hz/awoolt/version"
	"github.com/openbao/openbao/api/v2"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var rootCmd = &cobra.Command{
	Use:     "awoolt",
	Short:   "interactively browse vault/openbao in the terminal.",
	Version: version.Version,
	Run:     root,
}

var rootFlags struct {
	engine string
	path   string
}

func must(err error) {
	if err != nil {
		log.Fatal("Error", "err", err)
	}
}

func init() {
	rootCmd.Flags().StringVarP(&rootFlags.engine, "engine", "e", "", "secret engine to use")
	rootCmd.Flags().StringVarP(&rootFlags.path, "path", "p", "", "secret path")

	must(viper.BindPFlags(rootCmd.Flags()))
	rootCmd.AddCommand(versionCmd)
}

func root(cmd *cobra.Command, _ []string) {
	config, err := config.Load("")
	if err != nil {
		log.Fatal("Failed to load config", "err", err)
	}
	if config.Engine == "" {
		log.Fatal("No engine specified :(")
	}

	apiConfig := api.DefaultConfig()
	client, err := api.NewClient(apiConfig)
	if err != nil {
		log.Fatal("Failed to create client", "err", err)
	}

	token, err := os.ReadFile(path.Join(xdg.Home, ".vault-token"))
	if err != nil {
		log.Fatal("Failed to read token. Login to vault first!", "err", err)
	}
	client.SetToken(string(token))

	path := vaultPath{config.Engine}
	if rootFlags.path != "" {
		path.Add(strings.Split(rootFlags.path, "/")...)
	}

	for {
		availableKeys, done, err := listSecretsSpinner(client, path)
		if err != nil {
			log.Fatal("Failed to list secrets", "err", err)
		}
		if done {
			break
		}

		nextPathElement, err := selectNextPathElement(path.String(), availableKeys)
		if err != nil {
			return
		}
		path.Add(nextPathElement)
	}

	secret, err := client.KVv2(path.Engine()).Get(cmd.Context(), path.Path())
	if err != nil {
		log.Fatal("Failed to get secret", "err", err)
	}
	if secret == nil {
		log.Fatal("Secret not found")
	}

	fmt.Printf("path: %s\n", path.Path())
	printSecret(secret)
}

func listSecrets(client *api.Client, path vaultPath) ([]string, bool, error) {
	secret, err := client.Logical().List(path.MetadataPath())
	if err != nil {
		return nil, false, err
	}
	if secret == nil {
		return nil, true, nil
	}
	keys, ok := secret.Data["keys"].([]any)
	if !ok {
		log.Fatal("Failed to convert keys", "keys", secret.Data["keys"])
	}
	availableKeys := make([]string, len(keys))
	for i, key := range keys {
		availableKeys[i] = key.(string)
	}
	return availableKeys, false, nil
}

func listSecretsSpinner(client *api.Client, path vaultPath) (availableKeys []string, done bool, err error) {
	serr := spinner.New().
		Title("Fetching secrets...").
		Action(func() {
			availableKeys, done, err = listSecrets(client, path)
		}).
		Run()
	if serr != nil {
		err = serr
	}
	return
}

func selectNextPathElement(path string, options []string) (string, error) {
	_, height, err := term.GetSize(os.Stdin.Fd())
	if err != nil {
		return "", err
	}
	var nextPath string
	err = huh.NewForm(
		huh.NewGroup(
			huh.NewSelect[string]().
				Title(path).
				Height(min(len(options)+2, height-2)).
				Options(huh.NewOptions(options...)...).
				Value(&nextPath),
		),
	).Run()
	return nextPath, err
}

func printSecret(s *api.KVSecret) {
	for k, v := range s.Data {
		fmt.Printf("%s: %s\n", k, v)
	}
}

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print the version info",
	Run: func(_ *cobra.Command, _ []string) {
		fmt.Printf("Version: %s\n", version.Version)
		fmt.Printf("Commit: %s\n", version.Commit)
		fmt.Printf("Date: %s\n", version.Date)
		fmt.Printf("BuiltBy: %s\n", version.BuiltBy)
	},
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}
