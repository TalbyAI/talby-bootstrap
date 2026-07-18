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
