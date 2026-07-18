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
				root, err := repositoryRoot(ctx)
				if err != nil {
					return err
				}
				result, err := service.Sync(ctx, installsvc.SyncRequest{Root: root})
				if err != nil {
					return err
				}
				if opts.output == outputJSON {
					return json.NewEncoder(stdout).Encode(resultEnvelope("sync succeeded", result))
				}
				return writeResult(stdout, result)
			}

			ref, err := parseSourceRef(args[0])
			if err != nil {
				return err
			}
			root, err := repositoryRoot(ctx)
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
				return json.NewEncoder(stdout).Encode(resultEnvelope(message, result))
			}
			return writeResult(stdout, result)
		},
	}

	cmd.Flags().StringVar(&artifact, "artifact", "", "artifact to install")
	cmd.Flags().BoolVar(&declareOnly, "declare-only", false, "declare artifact intent without materializing files")
	return cmd
}

func resultEnvelope(message string, result installsvc.Result) app.Result {
	details := map[string]any{
		"operation":      result.Operation,
		"outcome":        result.Outcome,
		"artifact_count": result.ArtifactCount,
	}
	if len(result.Changes) != 0 {
		details["changes"] = result.Changes
	}
	if len(result.Conflicts) != 0 {
		details["conflicts"] = result.Conflicts
	}
	return app.Result{Code: app.ExitSuccess, Message: message, Details: details}
}

func writeResult(stdout io.Writer, result installsvc.Result) error {
	if result.Outcome == installsvc.OutcomeNoOp {
		_, err := fmt.Fprintf(stdout, "%s: no changes (%d artifacts)\n", result.Operation, result.ArtifactCount)
		return err
	}
	if _, err := fmt.Fprintf(stdout, "%s: applied %d changes (%d artifacts)\n", result.Operation, len(result.Changes), result.ArtifactCount); err != nil {
		return err
	}
	for _, change := range result.Changes {
		fields := []string{string(change.Kind), change.Source.Type + ":" + change.Source.Locator}
		if change.SourceVersion != "" {
			fields = append(fields, "source_version="+change.SourceVersion)
		}
		if change.Artifact != "" {
			fields = append(fields, change.Artifact)
		}
		if change.Path != "" {
			fields = append(fields, change.Path)
		}
		if change.OwnershipKind != "" {
			fields = append(fields, "ownership_kind="+string(change.OwnershipKind))
		}
		if _, err := fmt.Fprintln(stdout, strings.Join(fields, " ")); err != nil {
			return err
		}
	}
	return nil
}

func parseSourceRef(raw string) (source.Ref, error) {
	parsed, err := repositorystate.ParseSourceReference(raw)
	if err != nil {
		return source.Ref{}, err
	}

	return source.Ref{
		Type:    parsed.Type,
		Locator: parsed.Locator,
	}, nil
}

func repositoryRoot(ctx context.Context) (string, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return "", err
	}

	cmd := exec.CommandContext(ctx, "git", "rev-parse", "--show-toplevel")
	cmd.Dir = cwd
	output, err := cmd.Output()
	if err != nil {
		return cwd, nil //nolint:nilerr // fall back to cwd outside a git repository
	}

	root := strings.TrimSpace(string(output))
	if root == "" {
		return cwd, nil
	}
	return root, nil
}
