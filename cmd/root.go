package cmd

import (
	"context"
	"fmt"
	"os"
	"path"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/fang/v2"
	"charm.land/log/v2"
	"github.com/adrg/xdg"
	"github.com/jon4hz/awoolt/config"
	"github.com/jon4hz/awoolt/version"
	"github.com/openbao/openbao/api/v2"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var rootCmd = &cobra.Command{
	Use:     "awoolt",
	Short:   "A simple TUI for your openbao KV engines.",
	Version: version.Version,
	CompletionOptions: cobra.CompletionOptions{
		HiddenDefaultCmd: true,
	},
	Run: root,
}

var rootFlags struct {
	engine string
	path   string
	fields []string
}

func must(err error) {
	if err != nil {
		log.Fatal("Error", "err", err)
	}
}

func init() {
	rootCmd.Flags().StringVarP(&rootFlags.engine, "engine", "e", "", "secret engine to use")
	rootCmd.Flags().StringVarP(&rootFlags.path, "path", "p", "", "secret path")
	rootCmd.Flags().StringSliceVarP(&rootFlags.fields, "fields", "f", nil, "fields to display")

	must(viper.BindPFlags(rootCmd.Flags()))
	rootCmd.AddCommand(versionCmd)
}

// Execute runs the root command.
func Execute(ctx context.Context) error {
	return fang.Execute(ctx, rootCmd)
}

func root(_ *cobra.Command, _ []string) {
	config, err := config.Load("")
	if err != nil {
		log.Fatal("Failed to load config", "err", err)
	}
	if config.Engine == "" {
		log.Fatal("No engine specified :(")
	}

	client, err := newVaultClient()
	if err != nil {
		log.Fatal("Error", "err", err)
	}

	path := vaultPath{config.Engine}
	if p := rootFlags.path; p != "" {
		p = strings.TrimSuffix(p, "/")
		path.Add(strings.Split(p, "/")...)
	}

	m := newModel(client, path, rootFlags.fields)
	finalModel, err := tea.NewProgram(m).Run()
	if err != nil {
		log.Fatal("Error", "err", err)
	}
	if m, ok := finalModel.(model); ok {
		fmt.Print(m.output())
	}
}

// newVaultClient creates a vault client authenticated with the token from
// ~/.vault-token.
func newVaultClient() (*api.Client, error) {
	client, err := api.NewClient(api.DefaultConfig())
	if err != nil {
		return nil, fmt.Errorf("failed to create client: %w", err)
	}
	token, err := os.ReadFile(path.Join(xdg.Home, ".vault-token"))
	if err != nil {
		return nil, fmt.Errorf("failed to read token, login to vault first: %w", err)
	}
	client.SetToken(string(token))
	return client, nil
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
