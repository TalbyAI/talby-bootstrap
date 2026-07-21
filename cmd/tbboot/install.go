package tbboot

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
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
	var dryRun bool
	var prune bool
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
				result, err := service.Sync(ctx, installsvc.SyncRequest{Root: root, DryRun: dryRun, Prune: prune})
				if err != nil {
					return err
				}
				if opts.output == outputJSON {
					return json.NewEncoder(stdout).Encode(resultEnvelope("sync succeeded", result))
				}
				return writeResult(stdout, result)
			}
			if prune {
				return fmt.Errorf("prune requires targetless install")
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
				DryRun:      dryRun,
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
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "report changes without writing files or state")
	cmd.Flags().BoolVar(&prune, "prune", false, "remove unchanged files for artifacts absent from the complete desired state")
	return cmd
}

func resultEnvelope(message string, result installsvc.Result) app.Result {
	details := map[string]any{
		"operation":      result.Operation,
		"outcome":        result.Outcome,
		"dry_run":        result.DryRun,
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
	label := "applied"
	if result.Outcome == installsvc.OutcomePlanned {
		label = "planned"
	}
	if _, err := fmt.Fprintf(stdout, "%s: %s %d changes (%d artifacts)\n", result.Operation, label, len(result.Changes), result.ArtifactCount); err != nil {
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

type gitRunner func(context.Context, string) ([]byte, []byte, error)

const nonRepositoryGitDiagnostic = "fatal: not a git repository (or any of the parent directories): .git"

func repositoryRoot(ctx context.Context) (string, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return "", err
	}
	return repositoryRootAt(ctx, cwd, runGit)
}

func runGit(ctx context.Context, cwd string) ([]byte, []byte, error) {
	cmd := exec.CommandContext(ctx, "git", "rev-parse", "--show-toplevel")
	cmd.Dir = cwd
	cmd.Env = stableGitEnvironment(os.Environ())
	stdout, err := cmd.Output()
	if err == nil {
		return stdout, nil, nil
	}
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		return nil, exitErr.Stderr, err
	}
	return nil, nil, err
}

func stableGitEnvironment(environment []string) []string {
	filtered := make([]string, 0, len(environment)+1)
	for _, value := range environment {
		name, _, _ := strings.Cut(value, "=")
		if !isLCAllEnvironmentName(name) {
			filtered = append(filtered, value)
		}
	}
	return append(filtered, "LC_ALL=C")
}

func isLCAllEnvironmentName(name string) bool {
	if runtime.GOOS == "windows" {
		return strings.EqualFold(name, "LC_ALL")
	}
	return name == "LC_ALL"
}

func repositoryRootAt(ctx context.Context, cwd string, run gitRunner) (string, error) {
	stdout, stderr, err := run(ctx, cwd)
	if err != nil {
		if strings.TrimSpace(string(stderr)) != nonRepositoryGitDiagnostic {
			return "", fmt.Errorf("discover repository root: %w", err)
		}
		return canonicalPath(cwd)
	}

	root := strings.TrimSuffix(string(stdout), "\n")
	root = strings.TrimSuffix(root, "\r")
	if root == "" || strings.ContainsAny(root, "\r\n") {
		return "", fmt.Errorf("git returned malformed repository root")
	}
	return canonicalPath(root)
}

func canonicalPath(value string) (string, error) {
	absolute, err := filepath.Abs(value)
	if err != nil {
		return "", err
	}
	return filepath.EvalSymlinks(filepath.Clean(absolute))
}
