package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/rendis/senda/internal/openapi"
)

func main() {
	if len(os.Args) < 2 {
		fatalf("usage: go run ./cmd/openapi <generate-docs|convert|validate> [flags]")
	}

	switch os.Args[1] {
	case "generate-docs":
		if err := cmdGenerateDocs(os.Args[2:]); err != nil {
			fatalf("%v", err)
		}
	case "convert":
		if err := cmdConvert(os.Args[2:]); err != nil {
			fatalf("%v", err)
		}
	case "validate":
		if err := cmdValidate(os.Args[2:]); err != nil {
			fatalf("%v", err)
		}
	default:
		fatalf("unknown command %q", os.Args[1])
	}
}

func cmdGenerateDocs(args []string) error {
	fs := flag.NewFlagSet("generate-docs", flag.ContinueOnError)
	out := fs.String("out", "cmd/senda/openapi_generated.go", "output Go file")
	if err := fs.Parse(args); err != nil {
		return err
	}

	routes, err := openapi.RegisteredRoutes()
	if err != nil {
		return err
	}
	content := openapi.GenerateSwagDocsContent(routes)
	return os.WriteFile(*out, []byte(content), 0o644)
}

func cmdConvert(args []string) error {
	fs := flag.NewFlagSet("convert", flag.ContinueOnError)
	swaggerPath := fs.String("swagger", "", "input swagger.yaml path")
	out := fs.String("out", "", "output openapi.yaml path")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *swaggerPath == "" || *out == "" {
		return fmt.Errorf("--swagger and --out are required")
	}
	return openapi.ConvertSwaggerToOpenAPI(*swaggerPath, *out)
}

func cmdValidate(args []string) error {
	fs := flag.NewFlagSet("validate", flag.ContinueOnError)
	specPath := fs.String("spec", "", "openapi spec path")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *specPath == "" {
		return fmt.Errorf("--spec is required")
	}

	doc, err := openapi.LoadOpenAPI(*specPath)
	if err != nil {
		return err
	}
	routes, err := openapi.RegisteredRoutes()
	if err != nil {
		return err
	}
	return openapi.ValidateRouteCoverage(doc, routes)
}

func fatalf(format string, args ...any) {
	fmt.Fprintf(os.Stderr, format+"\n", args...)
	os.Exit(1)
}
