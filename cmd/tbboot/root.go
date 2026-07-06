package tbboot

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"

	"github.com/spf13/cobra"
	"github.com/talby/talby-bootstrap/internal/app"
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
		return int(app.ExitOperationalOrValidationError)
	}
	return int(app.ExitSuccess)
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
		placeholderCommand(ctx, opts, stdout, "install", []string{"i"}, "Install or sync artifacts"),
		placeholderCommand(ctx, opts, stdout, "upgrade", nil, "Upgrade declared artifacts"),
		placeholderCommand(ctx, opts, stdout, "search", nil, "Search configured catalogs"),
		placeholderCommand(ctx, opts, stdout, "logs", nil, "Replay recorded operations"),
		catalogCommand(ctx, opts, stdout),
	)
	return root
}

func catalogCommand(ctx context.Context, opts *options, stdout io.Writer) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "catalog",
		Short: "Manage catalogs",
	}
	cmd.AddCommand(
		placeholderCommand(ctx, opts, stdout, "add", nil, "Add a catalog"),
		placeholderCommand(ctx, opts, stdout, "list", nil, "List catalogs"),
		placeholderCommand(ctx, opts, stdout, "refresh", nil, "Refresh catalog caches"),
		placeholderCommand(ctx, opts, stdout, "remove", nil, "Remove a catalog"),
	)
	return cmd
}

func placeholderCommand(ctx context.Context, opts *options, stdout io.Writer, use string, aliases []string, short string) *cobra.Command {
	return &cobra.Command{
		Use:     use,
		Aliases: aliases,
		Short:   short,
		RunE: func(cmd *cobra.Command, args []string) error {
			result := app.Success("not implemented")
			if opts.output == outputJSON {
				return json.NewEncoder(stdout).Encode(result)
			}
			_, err := fmt.Fprintln(stdout, result.Message)
			return err
		},
	}
}
