package main

import (
	"fmt"
	"os"
	"path"
	"strings"

	"github.com/adrg/xdg"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/log"
	"github.com/jon4hz/awoolt/config"
	"github.com/jon4hz/awoolt/version"
	mcobra "github.com/muesli/mango-cobra"
	"github.com/muesli/roff"
	"github.com/openbao/openbao/api/v2"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var rootCmd = &cobra.Command{
	Use:     "awoolt",
	Short:   "interactively browse vault/openbao in the terminal.",
	Version: version.Version,
	CompletionOptions: cobra.CompletionOptions{
		HiddenDefaultCmd: true,
	},
	Run: root,
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
	rootCmd.AddCommand(versionCmd, manCmd)
}

func root(_ *cobra.Command, _ []string) {
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
	if p := rootFlags.path; p != "" {
		p = strings.TrimSuffix(p, "/")
		path.Add(strings.Split(p, "/")...)
	}

	m := newModel(client, path)
	if _, err := tea.NewProgram(m).Run(); err != nil {
		log.Fatal("Error", "err", err)
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

var manCmd = &cobra.Command{
	Use:                   "man",
	Short:                 "generates the manpages",
	SilenceUsage:          true,
	DisableFlagsInUseLine: true,
	Hidden:                true,
	Args:                  cobra.NoArgs,
	RunE: func(_ *cobra.Command, _ []string) error {
		manPage, err := mcobra.NewManPage(1, rootCmd)
		if err != nil {
			return err
		}
		_, err = fmt.Fprint(os.Stdout, manPage.Build(roff.NewDocument()))
		return err
	},
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}
