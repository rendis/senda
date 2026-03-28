package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/senda-app/senda/internal/teststack"
)

func cmdStack(args []string) error {
	if len(args) == 0 {
		return errors.New("stack subcommand required: up or down")
	}

	switch args[0] {
	case "up":
		return cmdStackUp(args[1:])
	case "down":
		return cmdStackDown(args[1:])
	default:
		return fmt.Errorf("unknown stack subcommand: %s", args[0])
	}
}

func cmdStackUp(args []string) error {
	fs := flag.NewFlagSet("stack up", flag.ContinueOnError)
	mode := fs.String("mode", "pr", "stack mode: pr or nightly")
	out := fs.String("out", "", "path to env-report json")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *out == "" {
		return errors.New("--out is required")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()

	root, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("resolve working directory: %w", err)
	}

	_, err = teststack.Up(ctx, teststack.Options{
		ProjectRoot: root,
		Mode:        teststack.Mode(*mode),
		OutPath:     *out,
	})
	return err
}

func cmdStackDown(args []string) error {
	fs := flag.NewFlagSet("stack down", flag.ContinueOnError)
	out := fs.String("out", "", "path to env-report json")
	if err := fs.Parse(args); err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	return teststack.Down(ctx, *out)
}
