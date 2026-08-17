package cmd

import (
	"errors"
	"fmt"
	"path"
	"sort"
	"strings"

	"charm.land/huh/v2"
	"charm.land/log/v2"
	"github.com/jon4hz/awoolt/config"
	"github.com/jon4hz/awoolt/put"
	"github.com/samber/lo"
	"github.com/spf13/cobra"
)

var putCmd = &cobra.Command{
	Use:   "put [path]...",
	Short: "Create new secrets using an interactive form",
	Long: `Create one or more secrets using an interactive form.

Secret paths can be passed as arguments to create multiple secrets at once,
otherwise the form asks for a path and whether to add another secret.
The fields of each secret can be predefined with --fields, so every secret
uses the same form. Selected fields can additionally be stored as custom
metadata of the secret. Note that custom metadata is not encrypted.`,
	Example: `  awoolt put -e kv app1 app2 app3 -f username,password -m username`,
	Run:     runPut,
}

var putFlags struct {
	engine         string
	path           string
	fields         []string
	metadataFields []string
}

func init() {
	putCmd.Flags().StringVarP(&putFlags.engine, "engine", "e", "", "secret engine to use")
	putCmd.Flags().StringVarP(&putFlags.path, "path", "p", "", "base path prepended to every secret path")
	putCmd.Flags().StringSliceVarP(&putFlags.fields, "fields", "f", nil, "fields every secret form asks for")
	putCmd.Flags().StringSliceVarP(&putFlags.metadataFields, "metadata-fields", "m", nil, "fields that are also stored as custom metadata (preselects the form checkboxes)")
	rootCmd.AddCommand(putCmd)
}

func runPut(cmd *cobra.Command, args []string) {
	engine := putFlags.engine
	if engine == "" {
		config, err := config.Load("")
		if err != nil {
			log.Fatal("Failed to load config", "err", err)
		}
		engine = config.Engine
	}
	if engine == "" {
		log.Fatal("No engine specified :(")
	}

	fields := putFlags.fields
	if len(fields) == 0 {
		var err error
		if fields, err = askFields(); err != nil {
			fatalForm(err)
		}
	}
	for _, field := range putFlags.metadataFields {
		if !lo.Contains(fields, field) {
			log.Fatal("Metadata field is not part of the secret fields", "field", field)
		}
	}

	secrets, err := collectSecrets(args, fields)
	if err != nil {
		fatalForm(err)
	}

	if ok, err := confirmWrite(engine, secrets); err != nil {
		fatalForm(err)
	} else if !ok {
		log.Warn("Aborted, nothing written")
		return
	}

	client, err := newVaultClient()
	if err != nil {
		log.Fatal("Failed to create client", "err", err)
	}
	if err := put.WriteAll(cmd.Context(), client.KVv2(engine), secrets); err != nil {
		log.Fatal("Failed to write secrets", "err", err)
	}
	for _, secret := range secrets {
		fmt.Printf("✓ %s\n", path.Join(engine, secret.Path))
	}
}

func fatalForm(err error) {
	if errors.Is(err, huh.ErrUserAborted) {
		log.Fatal("Aborted, nothing written")
	}
	log.Fatal("Error", "err", err)
}

// askFields asks for the fields of the secrets if none were given on the cli.
func askFields() ([]string, error) {
	var input string
	form := huh.NewForm(huh.NewGroup(
		huh.NewInput().
			Title("Fields").
			Description("comma separated list of fields every secret should have").
			Placeholder("username,password").
			Validate(validateFields).
			Value(&input),
	))
	if err := form.Run(); err != nil {
		return nil, err
	}
	return splitFields(input), nil
}

func validateFields(s string) error {
	if len(splitFields(s)) == 0 {
		return errors.New("at least one field is required")
	}
	return nil
}

func splitFields(s string) []string {
	return lo.Compact(lo.Map(strings.Split(s, ","), func(f string, _ int) string {
		return strings.TrimSpace(f)
	}))
}

// collectSecrets runs a form per secret. If no paths are given, the form also
// asks for the path and whether another secret should be added.
func collectSecrets(paths, fields []string) ([]put.Secret, error) {
	interactive := len(paths) == 0
	var secrets []put.Secret
	for i := 0; interactive || i < len(paths); i++ {
		var secretPath string
		if !interactive {
			secretPath = paths[i]
		}
		secret, addAnother, err := secretForm(secretPath, fields, interactive)
		if err != nil {
			return nil, err
		}
		secret.Path = path.Join(putFlags.path, secret.Path)
		secrets = append(secrets, secret)
		if interactive && !addAnother {
			break
		}
	}
	return secrets, nil
}

func secretForm(secretPath string, fields []string, interactive bool) (put.Secret, bool, error) {
	inputs := make([]huh.Field, 0, len(fields)+3)

	if interactive {
		inputs = append(inputs, huh.NewInput().
			Title("Path").
			Description("path of the secret").
			Validate(func(s string) error {
				if strings.TrimSpace(s) == "" {
					return errors.New("path must not be empty")
				}
				return nil
			}).
			Value(&secretPath),
		)
	}

	values := make([]string, len(fields))
	for i, field := range fields {
		inputs = append(inputs, huh.NewInput().Title(field).Value(&values[i]))
	}

	options := make([]huh.Option[string], len(fields))
	for i, field := range fields {
		options[i] = huh.NewOption(field, field).Selected(lo.Contains(putFlags.metadataFields, field))
	}
	var metadataFields []string
	inputs = append(inputs, huh.NewMultiSelect[string]().
		Title("Custom metadata").
		Description("fields additionally stored as unencrypted custom metadata").
		Options(options...).
		// huh v2.0.3 sizes the options viewport as max(1, height)-yoffset, so
		// without an explicit height covering the title and description the
		// options render into a zero-height viewport and cannot be selected.
		Height(len(fields)+2).
		Value(&metadataFields),
	)

	var addAnother bool
	if interactive {
		inputs = append(inputs, huh.NewConfirm().Title("Add another secret?").Value(&addAnother))
	}

	group := huh.NewGroup(inputs...)
	if !interactive {
		group = group.Title(path.Join(putFlags.path, secretPath))
	}
	if err := huh.NewForm(group).Run(); err != nil {
		return put.Secret{}, false, err
	}

	data := make(map[string]string, len(fields))
	for i, field := range fields {
		if values[i] != "" {
			data[field] = values[i]
		}
	}
	// drop metadata fields whose value was left empty
	metadataFields = lo.Filter(metadataFields, func(field string, _ int) bool {
		_, ok := data[field]
		return ok
	})

	return put.Secret{Path: strings.TrimSpace(secretPath), Data: data, MetadataFields: metadataFields}, addAnother, nil
}

func confirmWrite(engine string, secrets []put.Secret) (bool, error) {
	var summary strings.Builder
	for _, secret := range secrets {
		fmt.Fprintf(&summary, "%s\n", path.Join(engine, secret.Path))
		fields := lo.Keys(secret.Data)
		sort.Strings(fields)
		for _, field := range fields {
			if lo.Contains(secret.MetadataFields, field) {
				fmt.Fprintf(&summary, "  %s (+metadata)\n", field)
			} else {
				fmt.Fprintf(&summary, "  %s\n", field)
			}
		}
	}

	ok := true
	form := huh.NewForm(huh.NewGroup(
		huh.NewConfirm().
			Title(fmt.Sprintf("Write %d secret(s)?", len(secrets))).
			Description(summary.String()).
			Value(&ok),
	))
	if err := form.Run(); err != nil {
		return false, err
	}
	return ok, nil
}
