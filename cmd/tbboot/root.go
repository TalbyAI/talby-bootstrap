package tbboot

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"slices"
	"strings"

	"github.com/spf13/cobra"
	"github.com/talby/talby-bootstrap/internal/app"
	installsvc "github.com/talby/talby-bootstrap/internal/install"
	"github.com/talby/talby-bootstrap/internal/repositorystate"
)

const (
	outputHuman = "human"
	outputJSON  = "json"
)

type options struct {
	output string
}

type recoveryObservationDetail struct {
	Path     string                 `json:"path"`
	Result   string                 `json:"result"`
	Expected recoveryExpectedDetail `json:"expected"`
	Owner    *recoveryOwnerDetail   `json:"owner,omitempty"`
}

type recoveryExpectedDetail struct {
	State  string `json:"state"`
	Digest string `json:"digest,omitempty"`
	Mode   uint32 `json:"mode,omitempty"`
}

type recoveryOwnerDetail struct {
	Source        repositorystate.SourceIdentity `json:"source"`
	SourceVersion string                         `json:"source_version"`
	Artifact      string                         `json:"artifact"`
}

func Execute() int {
	return execute(context.Background(), os.Args[1:], os.Stdout, os.Stderr)
}

func execute(ctx context.Context, args []string, stdout io.Writer, stderr io.Writer) int {
	opts := options{output: outputHuman}
	root := newRootCommand(ctx, &opts, stdout)
	root.SetArgs(args)
	root.SetOut(stdout)
	root.SetErr(stderr)
	if err := root.Execute(); err != nil {
		code := app.ExitOperationalOrValidationError

		var recovery installsvc.RecoveryConflictError
		if errors.As(err, &recovery) {
			code = app.ExitUserActionConflict
		}
		var userAction installsvc.UserActionError
		if errors.As(err, &userAction) {
			code = app.ExitUserActionConflict
		}
		var trust installsvc.TrustPolicyError
		if errors.As(err, &trust) {
			code = app.ExitTrustOrPolicyDenial
		}

		if opts.output == outputJSON {
			result := app.Result{Code: code, Message: err.Error()}
			if errors.As(err, &recovery) {
				result = app.Result{
					Code:    app.ExitUserActionConflict,
					Message: recovery.Error(),
					Details: map[string]any{
						"recovery_code": repositorystate.RecoveryCodeRollbackIncomplete,
						"observations":  recoveryDetails(recovery.Observations),
					},
				}
			}
			if errors.As(err, &userAction) {
				result = resultEnvelope(err.Error(), userAction.Result)
				result.Code = code
			}
			if errors.As(err, &trust) {
				result.Details = map[string]any{"denied_sources": sortedDeniedSources(trust.Denied)}
			}
			_ = json.NewEncoder(stderr).Encode(result)
			return int(code)
		}
		if errors.As(err, &recovery) {
			for _, observation := range recovery.Observations {
				_, _ = fmt.Fprintf(stderr, "%s %s\n", repositorystate.RecoveryCodeRollbackIncomplete, observation.Path)
			}
			return int(code)
		}
		if errors.As(err, &userAction) {
			for _, conflict := range userAction.Result.Conflicts {
				_, _ = fmt.Fprintf(stderr, "%s %s:%s %s %s\n", conflict.Kind, conflict.Source.Type, conflict.Source.Locator, conflict.Artifact, strings.Join(conflict.Paths, ", "))
			}
			return int(code)
		}
		_, _ = fmt.Fprintln(stderr, err)
		return int(code)
	}
	return int(app.ExitSuccess)
}

func recoveryDetails(observations []repositorystate.RecoveryObservation) []recoveryObservationDetail {
	details := make([]recoveryObservationDetail, 0, len(observations))
	for _, observation := range observations {
		detail := recoveryObservationDetail{
			Path:   observation.Path,
			Result: observation.Result,
			Expected: recoveryExpectedDetail{
				State:  observation.ExpectedState,
				Digest: observation.Digest,
				Mode:   observation.Mode,
			},
		}
		if observation.Owner != nil {
			detail.Owner = &recoveryOwnerDetail{
				Source:        observation.Owner.Source,
				SourceVersion: observation.Owner.ResolvedVersion,
				Artifact:      observation.Owner.Artifact,
			}
		}
		details = append(details, detail)
	}
	return details
}

func sortedDeniedSources(values []repositorystate.SourceIdentity) []repositorystate.SourceIdentity {
	values = append([]repositorystate.SourceIdentity(nil), values...)
	slices.SortFunc(values, func(a, b repositorystate.SourceIdentity) int {
		return strings.Compare(repositorystate.SourceIdentityKey(a), repositorystate.SourceIdentityKey(b))
	})
	return slices.CompactFunc(values, func(a, b repositorystate.SourceIdentity) bool { return a == b })
}

func newRootCommand(ctx context.Context, opts *options, stdout io.Writer) *cobra.Command {
	root := &cobra.Command{
		Use:           "tbboot",
		Short:         "Reconcile reusable repository artifacts",
		SilenceUsage:  true,
		SilenceErrors: true,
	}
	root.PersistentFlags().StringVar(&opts.output, "output", outputHuman, "output mode: human or json")
	root.PersistentPreRunE = func(cmd *cobra.Command, args []string) error {
		switch opts.output {
		case outputHuman, outputJSON:
			return nil
		default:
			return fmt.Errorf("unsupported output mode %q", opts.output)
		}
	}
	root.AddCommand(
		installCommand(ctx, opts, stdout),
	)
	return root
}
