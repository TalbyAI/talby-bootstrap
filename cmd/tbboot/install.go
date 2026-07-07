package tbboot

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"

	"github.com/spf13/cobra"
	"github.com/talby/talby-bootstrap/internal/app"
	installsvc "github.com/talby/talby-bootstrap/internal/install"
	"github.com/talby/talby-bootstrap/internal/repositorystate"
	"github.com/talby/talby-bootstrap/internal/source"
	sourcefile "github.com/talby/talby-bootstrap/internal/source/file"
)

type sourceIdentityJSON struct {
	Type    string `json:"type"`
	Name    string `json:"name"`
	Version string `json:"version"`
}

type artifactDescriptorJSON struct {
	Name    string `json:"name"`
	Version string `json:"version"`
	Path    string `json:"path"`
}

func installCommand(ctx context.Context, opts *options, stdout io.Writer) *cobra.Command {
	var artifact string
	var declareOnly bool
	service := installsvc.NewService(
		source.NewStaticRegistry(map[string]source.Source{
			"file": sourcefile.New(),
		}),
		repositorystate.NewStore(),
	)

	cmd := &cobra.Command{
		Use:     "install [<source>]",
		Aliases: []string{"i"},
		Short:   "Install or sync artifacts",
		Args:    cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				if declareOnly {
					return fmt.Errorf("declare-only install requires an explicit <source>")
				}
				result := app.Success("sync not implemented")
				if opts.output == outputJSON {
					return json.NewEncoder(stdout).Encode(result)
				}
				_, err := fmt.Fprintln(stdout, result.Message)
				return err
			}

			ref, err := parseSourceRef(args[0])
			if err != nil {
				return err
			}
			root, err := repositoryRoot()
			if err != nil {
				return err
			}
			result, err := service.Install(ctx, installsvc.Request{
				Root:        root,
				Source:      ref,
				Artifact:    artifact,
				DeclareOnly: declareOnly,
			})
			if err != nil {
				return err
			}

			if opts.output == outputJSON {
				message := "install succeeded"
				if declareOnly {
					message = "declare-only succeeded"
				}
				envelope := app.Success(message)
				envelope.Details = map[string]any{
					"source":   mapSourceIdentity(result.Source),
					"artifact": mapArtifactDescriptor(result.Artifact),
				}
				if declareOnly {
					envelope.Details["change"] = result.Change
				}
				return json.NewEncoder(stdout).Encode(envelope)
			}

			if declareOnly {
				if result.Change == installsvc.ChangeNoOp {
					_, err = fmt.Fprintf(stdout, "artifact %s from %s is already declared\n", result.Artifact.Name, result.Source.Name)
					return err
				}
				_, err = fmt.Fprintf(stdout, "declared artifact %s from %s\n", result.Artifact.Name, result.Source.Name)
				return err
			}

			_, err = fmt.Fprintf(stdout, "selected artifact %s from %s\n", result.Artifact.Name, result.Source.Name)
			return err
		},
	}

	cmd.Flags().StringVar(&artifact, "artifact", "", "artifact to install")
	cmd.Flags().BoolVar(&declareOnly, "declare-only", false, "declare artifact intent without materializing files")
	return cmd
}

func mapSourceIdentity(identity source.Identity) sourceIdentityJSON {
	return sourceIdentityJSON{
		Type:    identity.Type,
		Name:    identity.Name,
		Version: identity.Version,
	}
}

func mapArtifactDescriptor(artifact source.ArtifactDescriptor) artifactDescriptorJSON {
	return artifactDescriptorJSON{
		Name:    artifact.Name,
		Version: artifact.Version,
		Path:    artifact.Path,
	}
}

func parseSourceRef(raw string) (source.Ref, error) {
	sourceType, locator, ok := strings.Cut(raw, ":")
	if !ok || sourceType == "" || locator == "" {
		return source.Ref{}, fmt.Errorf("source must be formatted as <type>:<locator>")
	}

	return source.Ref{
		Type:    sourceType,
		Locator: locator,
	}, nil
}

func repositoryRoot() (string, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return "", err
	}

	cmd := exec.Command("git", "rev-parse", "--show-toplevel")
	cmd.Dir = cwd
	output, err := cmd.Output()
	if err != nil {
		return cwd, nil
	}

	root := strings.TrimSpace(string(output))
	if root == "" {
		return cwd, nil
	}
	return root, nil
}
