package cmd

import (
	"context"
	"errors"
	"os"
	"os/exec"

	"github.com/MetroStar/quartzctl/internal/privateaccess"
	"github.com/urfave/cli/v3"
)

// PrivateAccessCommand scopes kubectl, Helm and acceptance helpers to one relay session.
func PrivateAccessCommand(p *CommandParams) *cli.Command {
	return &cli.Command{
		Name: "private-access", Usage: "Run a command with a temporary verified private EKS kubeconfig (use -- before command arguments)",
		SkipFlagParsing: true,
		Action: func(ctx context.Context, command *cli.Command) (result error) {
			arguments := command.Args().Slice()
			if len(arguments) > 0 && arguments[0] == "--" {
				arguments = arguments[1:]
			}
			if len(arguments) == 0 {
				return errors.New("private-access requires a command")
			}
			if !p.Settings().Config.Aws.PrivateAccess.Enabled {
				return errors.New("aws.private_access.enabled must be true")
			}
			if _, err := p.Provider().Kubernetes(ctx); err != nil {
				return err
			}
			defer func() { result = errors.Join(result, p.closePrivateAccess()) }()
			// #nosec G204 -- explicitly requested local CLI command, separate argv; no shell or remote input.
			child := exec.CommandContext(ctx, arguments[0], arguments[1:]...)
			child.Stdin, child.Stdout, child.Stderr = os.Stdin, command.Root().Writer, command.Root().ErrWriter
			child.Env = append(os.Environ(), "KUBECONFIG="+os.Getenv(privateaccess.ConfigEnv), "KUBE_CONFIG_PATH="+os.Getenv(privateaccess.ConfigEnv),
				"KUBE_PROXY_URL="+os.Getenv(privateaccess.ProxyEnv))
			return child.Run()
		},
	}
}
