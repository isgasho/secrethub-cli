package secrethub

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/secrethub/secrethub-cli/internals/cli/clip"
	"github.com/secrethub/secrethub-cli/internals/cli/filemode"
	"github.com/secrethub/secrethub-cli/internals/cli/posix"
	"github.com/secrethub/secrethub-cli/internals/cli/ui"
	"github.com/secrethub/secrethub-cli/internals/cli/validation"
	"github.com/secrethub/secrethub-cli/internals/secrethub/tpl"

	"github.com/secrethub/secrethub-go/internals/errio"
	"github.com/secrethub/secrethub-go/pkg/secrethub"

	"github.com/docker/go-units"
)

// Errors
var (
	ErrUnknownTemplateVersion = errMain.Code("unknown_template_version").ErrorPref("unknown template version: '%s' supported versions are 1, 2 and latest")
)

// InjectCommand is a command to read a secret.
type InjectCommand struct {
	file                string
	fileMode            filemode.FileMode
	force               bool
	io                  ui.IO
	useClipboard        bool
	clearClipboardAfter time.Duration
	clipper             clip.Clipper
	newClient           newClientFunc
	templateVars        map[string]string
	templateVersion     string
}

// NewInjectCommand creates a new InjectCommand.
func NewInjectCommand(io ui.IO, newClient newClientFunc) *InjectCommand {
	return &InjectCommand{
		clipper:             clip.NewClipboard(),
		clearClipboardAfter: defaultClearClipboardAfter,
		io:                  io,
		newClient:           newClient,
		templateVars:        make(map[string]string),
	}
}

// Register adds a CommandClause and it's args and flags to a cli.App.
// Register adds args and flags.
func (cmd *InjectCommand) Register(r Registerer) {
	clause := r.Command("inject", "Inject secrets into a template.")
	clause.Flag(
		"clip",
		fmt.Sprintf(
			"Copy the injected template to the clipboard instead of stdout. The clipboard is automatically cleared after %s.",
			units.HumanDuration(cmd.clearClipboardAfter),
		),
	).Short('c').BoolVar(&cmd.useClipboard)
	clause.Flag("file", "Write the injected template to a file instead of stdout.").StringVar(&cmd.file)
	clause.Flag("file-mode", "Set filemode for the file if it does not yet exist. Defaults to 0600 (read and write for current user) and is ignored without the --file flag.").Default("0600").SetValue(&cmd.fileMode)
	clause.Flag("var", "Set variables to be used in templates.").Short('v').StringMapVar(&cmd.templateVars)
	clause.Flag("template-version", "The template syntax version to be used.").Default("latest").StringVar(&cmd.templateVersion)
	registerForceFlag(clause).BoolVar(&cmd.force)

	BindAction(clause, cmd.Run)
}

// Run handles the command with the options as specified in the command.
func (cmd *InjectCommand) Run() error {
	if cmd.useClipboard && cmd.file != "" {
		return ErrFlagsConflict("--clip and --file")
	}

	var err error

	if !cmd.io.Stdin().IsPiped() {
		return ErrNoDataOnStdin
	}

	raw, err := ioutil.ReadAll(cmd.io.Stdin())
	if err != nil {
		return errio.Error(err)
	}

	templateVars := make(map[string]string)

	osEnv, err := parseKeyValueStringsToMap(os.Environ())
	if err != nil {
		return errio.Error(err)
	}

	for k, v := range osEnv {
		if strings.HasPrefix(k, templateVarEnvVarPrefix) {
			k = strings.TrimPrefix(k, templateVarEnvVarPrefix)
			templateVars[k] = v
		}
	}

	for k, v := range cmd.templateVars {
		templateVars[k] = v
	}

	for k := range templateVars {
		if !validation.IsEnvarNamePosix(k) {
			return ErrInvalidTemplateVar(k)
		}
	}

	var parser tpl.Parser
	switch cmd.templateVersion {
	case "1":
		parser = tpl.NewV1Parser()
	case "2":
		parser = tpl.NewV2Parser()
	case "latest":
		parser = tpl.NewParser()
	default:
		return ErrUnknownTemplateVersion(cmd.templateVersion)
	}

	varTemplate, err := parser.Parse(string(raw))
	if err != nil {
		return errio.Error(err)
	}

	secretTemplate, err := varTemplate.InjectVars(templateVars)
	if err != nil {
		return err
	}

	secrets := make(map[string]string)

	var client secrethub.Client
	secretPaths := secretTemplate.Secrets()
	if len(secretPaths) > 0 {
		client, err = cmd.newClient()
		if err != nil {
			return errio.Error(err)
		}
	}

	for _, path := range secretPaths {
		secret, err := client.Secrets().Versions().GetWithData(path)
		if err != nil {
			return errio.Error(err)
		}
		secrets[path] = string(secret.Data)
	}

	injected, err := secretTemplate.InjectSecrets(secrets)
	if err != nil {
		return errio.Error(err)
	}

	out := []byte(injected)
	if cmd.useClipboard {
		err = WriteClipboardAutoClear(out, cmd.clearClipboardAfter, cmd.clipper)
		if err != nil {
			return errio.Error(err)
		}

		fmt.Fprintln(cmd.io.Stdout(), fmt.Sprintf("Copied injected template to clipboard. It will be cleared after %s.", units.HumanDuration(cmd.clearClipboardAfter)))
	} else if cmd.file != "" {
		_, err := os.Stat(cmd.file)
		if err == nil && !cmd.force {
			if cmd.io.Stdout().IsPiped() {
				return ErrFileAlreadyExists
			}

			confirmed, err := ui.AskYesNo(
				cmd.io,
				fmt.Sprintf(
					"File %s already exists, overwrite it?",
					cmd.file,
				),
				ui.DefaultNo,
			)
			if err != nil {
				return errio.Error(err)
			}

			if !confirmed {
				fmt.Fprintln(cmd.io.Stdout(), "Aborting.")
				return nil
			}
		}

		err = ioutil.WriteFile(cmd.file, posix.AddNewLine(out), cmd.fileMode.FileMode())
		if err != nil {
			return ErrCannotWrite(cmd.file, err)
		}

		absPath, err := filepath.Abs(cmd.file)
		if err != nil {
			return ErrCannotWrite(err)
		}

		fmt.Fprintf(cmd.io.Stdout(), "%s\n", absPath)
	} else {
		fmt.Fprintf(cmd.io.Stdout(), "%s", posix.AddNewLine(out))
	}

	return nil
}
