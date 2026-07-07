package tbboot

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"github.com/spf13/cobra"
	"github.com/talby/talby-bootstrap/internal/app"
	installsvc "github.com/talby/talby-bootstrap/internal/install"
	"github.com/talby/talby-bootstrap/internal/source"
	sourcefile "github.com/talby/talby-bootstrap/internal/source/file"
)

type installResultJSON struct {
	Source   sourceIdentityJSON     `json:"source"`
	Artifact artifactDescriptorJSON `json:"artifact"`
}

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
	service := installsvc.NewService(source.NewStaticRegistry(map[string]source.Source{
		"file": sourcefile.New(),
	}))

	cmd := &cobra.Command{
		Use:     "install [<source>]",
		Aliases: []string{"i"},
		Short:   "Install or sync artifacts",
		Args:    cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
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
			result, err := service.Install(ctx, installsvc.Request{
				Source:   ref,
				Artifact: artifact,
			})
			if err != nil {
				return err
			}

			if opts.output == outputJSON {
				envelope := app.Success("install succeeded")
				envelope.Details = map[string]any{
					"source":   mapSourceIdentity(result.Source),
					"artifact": mapArtifactDescriptor(result.Artifact),
				}
				return json.NewEncoder(stdout).Encode(envelope)
			}

			_, err = fmt.Fprintf(stdout, "selected artifact %s from %s\n", result.Artifact.Name, result.Source.Name)
			return err
		},
	}

	cmd.Flags().StringVar(&artifact, "artifact", "", "artifact to install")
	return cmd
}

func mapInstallResult(result installsvc.Result) installResultJSON {
	return installResultJSON{
		Source:   mapSourceIdentity(result.Source),
		Artifact: mapArtifactDescriptor(result.Artifact),
	}
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
