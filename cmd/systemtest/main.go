package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/csv"
	"encoding/json"
	"encoding/xml"
	"errors"
	"flag"
	"fmt"
	"image"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"math"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/sesv2"
	"github.com/rendis/senda/internal/adapter/testauth"
)

const (
	defaultJWTSecret       = "e2e-test-jwt-secret-at-least-32-characters-long"
	defaultIssuer          = "senda-system-test"
	defaultBootstrapEmail  = "superadmin@test.example.com"
	defaultSuperadmin      = "admin@senda.dev"
	defaultTenantAdmin     = "tenant-admin@senda.dev"
	defaultWorkspaceAdmin  = "workspace-admin@senda.dev"
	defaultWorkspaceEditor = "workspace-editor@senda.dev"
	defaultWorkspaceViewer = "workspace-viewer@senda.dev"
	defaultNoMember        = "no-member@senda.dev"
)

type RouteInfo struct {
	Route   string `json:"route"`
	Family  string `json:"family"`
	Scope   string `json:"scope"`
	Dynamic bool   `json:"dynamic"`
}

type ScreenManifest struct {
	Version        string         `json:"version"`
	MatrixDefaults MatrixDefaults `json:"matrixDefaults"`
	Screens        []ScreenSpec   `json:"screens"`
}

type MatrixDefaults struct {
	Role     []string `json:"role"`
	Locale   []string `json:"locale"`
	Viewport []string `json:"viewport"`
}

type ScreenSpec struct {
	Route         string   `json:"route"`
	Scope         []string `json:"scope"`
	Role          []string `json:"role"`
	Locale        []string `json:"locale"`
	Viewport      []string `json:"viewport"`
	Preconditions []string `json:"preconditions"`
	Actions       []string `json:"actions"`
	Assertions    []string `json:"assertions"`
	PencilFrameID string   `json:"pencilFrameId"`
	Critical      bool     `json:"critical"`
}

type VisualBaselineMap struct {
	Version string                `json:"version"`
	Entries []VisualBaselineEntry `json:"entries"`
}

type VisualBaselineEntry struct {
	Route         string `json:"route"`
	PencilFrameID string `json:"pencilFrameId"`
	Critical      bool   `json:"critical"`
}

type MatrixRow struct {
	Route         string
	RouteSlug     string
	Scope         string
	Role          string
	Locale        string
	Viewport      string
	Critical      bool
	PencilFrameID string
	Preconditions string
	Actions       string
	Assertions    string
}

type VisualDiffResult struct {
	Route          string  `json:"route"`
	Viewport       string  `json:"viewport"`
	Locale         string  `json:"locale"`
	Critical       bool    `json:"critical"`
	GoldenDiffPct  float64 `json:"goldenDiffPct"`
	PencilDiffPct  float64 `json:"pencilDiffPct"`
	ThresholdPct   float64 `json:"thresholdPct"`
	ActualPath     string  `json:"actualPath"`
	GoldenPath     string  `json:"goldenPath"`
	PencilPath     string  `json:"pencilPath"`
	Status         string  `json:"status"`
	FailureMessage string  `json:"failureMessage,omitempty"`
}

type stageResult struct {
	Name       string `json:"name"`
	Status     string `json:"status"`
	DurationMs int64  `json:"duration_ms"`
	LogPath    string `json:"log_path"`
}

func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(2)
	}

	var err error
	switch os.Args[1] {
	case "inventory":
		err = cmdInventory(os.Args[2:])
	case "validate-manifest":
		err = cmdValidateManifest(os.Args[2:])
	case "matrix":
		err = cmdMatrix(os.Args[2:])
	case "token":
		err = cmdToken(os.Args[2:])
	case "resolve-context":
		err = cmdResolveContext(os.Args[2:])
	case "keycloak-seed":
		err = cmdKeycloakSeed(os.Args[2:])
	case "seed-rbac":
		err = cmdSeedRBAC(os.Args[2:])
	case "aws-sim-create-identity":
		err = cmdAWSSimCreateIdentity(os.Args[2:])
	case "visual-diff":
		err = cmdVisualDiff(os.Args[2:])
	case "junit":
		err = cmdJunit(os.Args[2:])
	case "run-result":
		err = cmdRunResult(os.Args[2:])
	case "stack":
		err = cmdStack(os.Args[2:])
	default:
		usage()
		err = fmt.Errorf("unknown command: %s", os.Args[1])
	}

	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

func usage() {
	fmt.Println("systemtest commands:")
	fmt.Println("  inventory")
	fmt.Println("  validate-manifest")
	fmt.Println("  matrix")
	fmt.Println("  token")
	fmt.Println("  resolve-context")
	fmt.Println("  keycloak-seed")
	fmt.Println("  seed-rbac")
	fmt.Println("  aws-sim-create-identity")
	fmt.Println("  visual-diff")
	fmt.Println("  junit")
	fmt.Println("  run-result")
	fmt.Println("  stack <up|down>")
}

func cmdAWSSimCreateIdentity(args []string) error {
	fs := flag.NewFlagSet("aws-sim-create-identity", flag.ContinueOnError)
	endpoint := fs.String("endpoint", "", "aws-sim / MiniStack base endpoint")
	identity := fs.String("identity", "", "email or domain identity to create")
	region := fs.String("region", "us-east-1", "AWS region")
	accessKeyID := fs.String("access-key-id", "test", "AWS access key ID")
	secretAccessKey := fs.String("secret-access-key", "test", "AWS secret access key")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if strings.TrimSpace(*endpoint) == "" {
		return errors.New("--endpoint is required")
	}
	if strings.TrimSpace(*identity) == "" {
		return errors.New("--identity is required")
	}

	cfg, err := awsconfig.LoadDefaultConfig(context.Background(),
		awsconfig.WithRegion(*region),
		awsconfig.WithBaseEndpoint(*endpoint),
		awsconfig.WithCredentialsProvider(
			credentials.NewStaticCredentialsProvider(*accessKeyID, *secretAccessKey, ""),
		),
	)
	if err != nil {
		return fmt.Errorf("load aws config: %w", err)
	}

	client := sesv2.NewFromConfig(cfg, func(o *sesv2.Options) {
		o.BaseEndpoint = aws.String(*endpoint)
	})

	created := true
	if _, err := client.CreateEmailIdentity(context.Background(), &sesv2.CreateEmailIdentityInput{
		EmailIdentity: aws.String(*identity),
	}); err != nil {
		if !strings.Contains(strings.ToLower(err.Error()), "already exists") {
			return fmt.Errorf("create aws-sim identity %q: %w", *identity, err)
		}
		created = false
	}

	return json.NewEncoder(os.Stdout).Encode(map[string]any{
		"identity": *identity,
		"created":  created,
	})
}

func cmdInventory(args []string) error {
	fs := flag.NewFlagSet("inventory", flag.ContinueOnError)
	appDir := fs.String("app-dir", "web/src/app", "Next.js app directory")
	format := fs.String("format", "json", "json or csv")
	out := fs.String("out", "", "output file (stdout if empty)")
	if err := fs.Parse(args); err != nil {
		return err
	}

	routes, err := scanRoutes(*appDir)
	if err != nil {
		return err
	}

	var buf bytes.Buffer
	switch *format {
	case "json":
		enc := json.NewEncoder(&buf)
		enc.SetIndent("", "  ")
		if err := enc.Encode(routes); err != nil {
			return err
		}
	case "csv":
		w := csv.NewWriter(&buf)
		if err := w.Write([]string{"route", "family", "scope", "dynamic"}); err != nil {
			return err
		}
		for _, r := range routes {
			if err := w.Write([]string{r.Route, r.Family, r.Scope, strconv.FormatBool(r.Dynamic)}); err != nil {
				return err
			}
		}
		w.Flush()
		if err := w.Error(); err != nil {
			return err
		}
	default:
		return fmt.Errorf("unsupported format: %s", *format)
	}

	return writeOutput(*out, buf.Bytes())
}

func cmdValidateManifest(args []string) error {
	fs := flag.NewFlagSet("validate-manifest", flag.ContinueOnError)
	manifestPath := fs.String("manifest", "test/system/screen-manifest.json", "screen manifest path")
	baselineMapPath := fs.String("baseline-map", "test/system/visual-baseline-map.json", "visual baseline map path")
	appDir := fs.String("app-dir", "web/src/app", "Next.js app directory")
	if err := fs.Parse(args); err != nil {
		return err
	}

	manifest, err := loadManifest(*manifestPath)
	if err != nil {
		return err
	}
	baselines, err := loadBaselineMap(*baselineMapPath)
	if err != nil {
		return err
	}
	routes, err := scanRoutes(*appDir)
	if err != nil {
		return err
	}

	routeSet := make(map[string]struct{}, len(routes))
	for _, r := range routes {
		routeSet[r.Route] = struct{}{}
	}

	manifestByRoute := make(map[string]ScreenSpec, len(manifest.Screens))
	var validationErrors []string

	for _, s := range manifest.Screens {
		if _, ok := manifestByRoute[s.Route]; ok {
			validationErrors = append(validationErrors, fmt.Sprintf("duplicate route in manifest: %s", s.Route))
			continue
		}
		manifestByRoute[s.Route] = s

		if s.Route == "" {
			validationErrors = append(validationErrors, "manifest has empty route")
		}
		if len(s.Scope) == 0 {
			validationErrors = append(validationErrors, fmt.Sprintf("route %s has empty scope", s.Route))
		}
		if len(effectiveList(s.Role, manifest.MatrixDefaults.Role)) == 0 {
			validationErrors = append(validationErrors, fmt.Sprintf("route %s has empty role", s.Route))
		}
		if len(effectiveList(s.Locale, manifest.MatrixDefaults.Locale)) == 0 {
			validationErrors = append(validationErrors, fmt.Sprintf("route %s has empty locale", s.Route))
		}
		if len(effectiveList(s.Viewport, manifest.MatrixDefaults.Viewport)) == 0 {
			validationErrors = append(validationErrors, fmt.Sprintf("route %s has empty viewport", s.Route))
		}
		if len(s.Preconditions) == 0 {
			validationErrors = append(validationErrors, fmt.Sprintf("route %s has empty preconditions", s.Route))
		}
		if len(s.Actions) == 0 {
			validationErrors = append(validationErrors, fmt.Sprintf("route %s has empty actions", s.Route))
		}
		if len(s.Assertions) == 0 {
			validationErrors = append(validationErrors, fmt.Sprintf("route %s has empty assertions", s.Route))
		}
		if strings.TrimSpace(s.PencilFrameID) == "" {
			validationErrors = append(validationErrors, fmt.Sprintf("route %s has empty pencilFrameId", s.Route))
		}
	}

	for route := range routeSet {
		if _, ok := manifestByRoute[route]; !ok {
			validationErrors = append(validationErrors, fmt.Sprintf("route missing from manifest: %s", route))
		}
	}
	for route := range manifestByRoute {
		if _, ok := routeSet[route]; !ok {
			validationErrors = append(validationErrors, fmt.Sprintf("manifest route does not exist: %s", route))
		}
	}

	baselineByRoute := make(map[string]VisualBaselineEntry, len(baselines.Entries))
	for _, b := range baselines.Entries {
		if _, ok := baselineByRoute[b.Route]; ok {
			validationErrors = append(validationErrors, fmt.Sprintf("duplicate route in baseline map: %s", b.Route))
			continue
		}
		baselineByRoute[b.Route] = b
		if strings.TrimSpace(b.PencilFrameID) == "" {
			validationErrors = append(validationErrors, fmt.Sprintf("baseline route %s has empty pencilFrameId", b.Route))
		}
	}

	for route := range manifestByRoute {
		if _, ok := baselineByRoute[route]; !ok {
			validationErrors = append(validationErrors, fmt.Sprintf("route missing from baseline map: %s", route))
		}
	}
	for route := range baselineByRoute {
		if _, ok := manifestByRoute[route]; !ok {
			validationErrors = append(validationErrors, fmt.Sprintf("baseline route does not exist in manifest: %s", route))
		}
	}

	if len(validationErrors) > 0 {
		for _, msg := range validationErrors {
			fmt.Println("-", msg)
		}
		return fmt.Errorf("manifest validation failed: %d errors", len(validationErrors))
	}

	fmt.Printf("manifest validation passed: %d routes covered\n", len(routeSet))
	return nil
}

func cmdMatrix(args []string) (err error) {
	fs := flag.NewFlagSet("matrix", flag.ContinueOnError)
	manifestPath := fs.String("manifest", "test/system/screen-manifest.json", "screen manifest path")
	outPath := fs.String("out", "", "csv output path")
	format := fs.String("format", "csv", "csv or tsv")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *outPath == "" {
		return errors.New("--out is required")
	}

	manifest, err := loadManifest(*manifestPath)
	if err != nil {
		return err
	}

	rows := expandMatrix(manifest)
	if len(rows) == 0 {
		return errors.New("matrix expansion generated 0 rows")
	}

	if err := os.MkdirAll(filepath.Dir(*outPath), 0o755); err != nil {
		return err
	}
	f, err := os.Create(*outPath)
	if err != nil {
		return err
	}
	defer closeWithError(f, &err)

	switch *format {
	case "csv":
		w := csv.NewWriter(f)
		defer flushCSVWithError(w, &err)

		header := []string{"route", "route_slug", "scope", "role", "locale", "viewport", "critical", "pencil_frame_id", "preconditions", "actions", "assertions"}
		if err := w.Write(header); err != nil {
			return err
		}
		for _, r := range rows {
			if err := w.Write([]string{
				r.Route,
				r.RouteSlug,
				r.Scope,
				r.Role,
				r.Locale,
				r.Viewport,
				strconv.FormatBool(r.Critical),
				r.PencilFrameID,
				r.Preconditions,
				r.Actions,
				r.Assertions,
			}); err != nil {
				return err
			}
		}
	case "tsv":
		if _, err := f.WriteString("route\troute_slug\tscope\trole\tlocale\tviewport\tcritical\tpencil_frame_id\tpreconditions\tactions\tassertions\n"); err != nil {
			return err
		}
		for _, r := range rows {
			line := strings.Join([]string{
				r.Route,
				r.RouteSlug,
				r.Scope,
				r.Role,
				r.Locale,
				r.Viewport,
				strconv.FormatBool(r.Critical),
				r.PencilFrameID,
				r.Preconditions,
				r.Actions,
				r.Assertions,
			}, "\t") + "\n"
			if _, err := f.WriteString(line); err != nil {
				return err
			}
		}
	default:
		return fmt.Errorf("unsupported matrix format: %s", *format)
	}

	fmt.Printf("matrix rows written: %d\n", len(rows))
	return nil
}

func cmdToken(args []string) error {
	fs := flag.NewFlagSet("token", flag.ContinueOnError)
	email := fs.String("email", "admin@senda.dev", "OIDC email claim")
	subject := fs.String("subject", "", "OIDC sub claim (default email)")
	issuer := fs.String("issuer", defaultIssuer, "OIDC issuer claim")
	secret := fs.String("secret", envOrDefault("SENDA_E2E_JWT_SECRET", defaultJWTSecret), "HS256 secret")
	expiry := fs.Duration("expiry", time.Hour, "token expiry")
	if err := fs.Parse(args); err != nil {
		return err
	}

	sub := *subject
	if strings.TrimSpace(sub) == "" {
		sub = *email
	}
	token, err := testauth.GenerateToken(*email, sub, *issuer, *secret, *expiry)
	if err != nil {
		return err
	}
	fmt.Println(token)
	return nil
}

func cmdResolveContext(args []string) error {
	fs := flag.NewFlagSet("resolve-context", flag.ContinueOnError)
	baseURL := fs.String("base-url", "http://localhost:8090", "Senda API base URL")
	email := fs.String("email", defaultBootstrapEmail, "bootstrap superadmin email")
	issuer := fs.String("issuer", defaultIssuer, "OIDC issuer")
	secret := fs.String("secret", envOrDefault("SENDA_E2E_JWT_SECRET", defaultJWTSecret), "HS256 secret")
	tenantCode := fs.String("tenant-code", "test-corp", "tenant code")
	tenantName := fs.String("tenant-name", "Test Corp", "tenant name")
	workspaceCode := fs.String("workspace-code", "main", "workspace code")
	workspaceName := fs.String("workspace-name", "Main Workspace", "workspace name")
	templateSlug := fs.String("template-slug", "welcome-v1", "template slug")
	templateType := fs.String("template-type", "welcome-email", "template type slug")
	recipient := fs.String("recipient", "qa-recipient@senda.dev", "recipient email")
	out := fs.String("out", "", "output env file")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *out == "" {
		return errors.New("--out is required")
	}

	token, err := testauth.GenerateToken(*email, *email, *issuer, *secret, time.Hour)
	if err != nil {
		return err
	}

	client := &http.Client{Timeout: 20 * time.Second}

	if err := ensureOnboarding(client, *baseURL, token, *tenantCode, *tenantName); err != nil {
		return err
	}
	if err := ensureWorkspace(client, *baseURL, token, *tenantCode, *workspaceCode, *workspaceName); err != nil {
		return err
	}

	apiKey, err := createAPIKey(client, *baseURL, token, *tenantCode, *workspaceCode)
	if err != nil {
		return err
	}

	trackingID, err := latestTrackingID(client, *baseURL, token, *tenantCode, *workspaceCode)
	if err != nil {
		return err
	}
	if trackingID == "" {
		trackingID, err = sendOneEmail(client, *baseURL, apiKey, *tenantCode, *workspaceCode, *templateType, *recipient)
		if err != nil {
			// Keep context generation resilient when template fixtures are absent.
			// UI route tests can still navigate detail pages with a placeholder ID.
			trackingID = "missing-tracking-id"
		}
	}

	var b strings.Builder
	b.WriteString("TENANT_CODE=" + *tenantCode + "\n")
	b.WriteString("WORKSPACE_CODE=" + *workspaceCode + "\n")
	b.WriteString("TEMPLATE_SLUG=" + *templateSlug + "\n")
	b.WriteString("TEMPLATE_TYPE=" + *templateType + "\n")
	b.WriteString("TRACKING_ID=" + trackingID + "\n")
	b.WriteString("OIDC_TOKEN=" + token + "\n")
	b.WriteString("API_KEY=" + apiKey + "\n")
	b.WriteString("ADMIN_EMAIL=" + *email + "\n")

	if err := os.MkdirAll(filepath.Dir(*out), 0o755); err != nil {
		return err
	}
	if err := os.WriteFile(*out, []byte(b.String()), 0o644); err != nil {
		return err
	}

	fmt.Printf("context resolved: tenant=%s workspace=%s tracking_id=%s\n", *tenantCode, *workspaceCode, trackingID)
	return nil
}

func cmdKeycloakSeed(args []string) error {
	fs := flag.NewFlagSet("keycloak-seed", flag.ContinueOnError)
	baseURL := fs.String("base-url", "http://localhost:9090", "Keycloak base URL")
	realm := fs.String("realm", "senda", "target realm")
	adminUser := fs.String("admin-user", "admin", "admin username")
	adminPass := fs.String("admin-pass", "admin", "admin password")
	usersArg := fs.String("users", "no-member@senda.dev:admin", "comma-separated email:password list")
	if err := fs.Parse(args); err != nil {
		return err
	}

	adminToken, err := keycloakAdminToken(*baseURL, *adminUser, *adminPass)
	if err != nil {
		return err
	}

	pairs := strings.Split(strings.TrimSpace(*usersArg), ",")
	created := 0
	for _, pair := range pairs {
		pair = strings.TrimSpace(pair)
		if pair == "" {
			continue
		}
		chunks := strings.SplitN(pair, ":", 2)
		if len(chunks) != 2 {
			return fmt.Errorf("invalid --users entry %q, expected email:password", pair)
		}
		email := strings.TrimSpace(chunks[0])
		password := strings.TrimSpace(chunks[1])
		if email == "" || password == "" {
			return fmt.Errorf("invalid --users entry %q", pair)
		}

		exists, err := keycloakUserExists(*baseURL, *realm, adminToken, email)
		if err != nil {
			return err
		}
		if exists {
			continue
		}
		if err := keycloakCreateUser(*baseURL, *realm, adminToken, email, password); err != nil {
			return err
		}
		created++
	}

	fmt.Printf("keycloak users ensured, created=%d\n", created)
	return nil
}

func cmdSeedRBAC(args []string) error {
	fs := flag.NewFlagSet("seed-rbac", flag.ContinueOnError)
	baseURL := fs.String("base-url", "http://localhost:8090", "Senda API base URL")
	email := fs.String("email", defaultBootstrapEmail, "bootstrap superadmin email")
	issuer := fs.String("issuer", defaultIssuer, "OIDC issuer")
	secret := fs.String("secret", envOrDefault("SENDA_E2E_JWT_SECRET", defaultJWTSecret), "HS256 secret")
	tenantCode := fs.String("tenant-code", "test-corp", "tenant code")
	tenantName := fs.String("tenant-name", "Test Corp", "tenant name")
	workspaceCode := fs.String("workspace-code", "main", "workspace code")
	workspaceName := fs.String("workspace-name", "Main Workspace", "workspace name")
	superadminEmail := fs.String("superadmin-email", defaultSuperadmin, "superadmin user email")
	tenantAdminEmail := fs.String("tenant-admin-email", defaultTenantAdmin, "tenant admin user email")
	workspaceAdminEmail := fs.String("workspace-admin-email", defaultWorkspaceAdmin, "workspace admin user email")
	workspaceEditorEmail := fs.String("workspace-editor-email", defaultWorkspaceEditor, "workspace editor user email")
	workspaceViewerEmail := fs.String("workspace-viewer-email", defaultWorkspaceViewer, "workspace viewer user email")
	noMemberEmail := fs.String("no-member-email", defaultNoMember, "no-member user email")
	if err := fs.Parse(args); err != nil {
		return err
	}

	token, err := testauth.GenerateToken(*email, *email, *issuer, *secret, time.Hour)
	if err != nil {
		return err
	}

	client := &http.Client{Timeout: 20 * time.Second}
	if err := ensureOnboarding(client, *baseURL, token, *tenantCode, *tenantName); err != nil {
		return err
	}

	tenantID, err := getTenantID(client, *baseURL, token, *tenantCode)
	if err != nil {
		return err
	}

	superadminID, err := ensureMemberByEmail(client, *baseURL, token, *superadminEmail)
	if err != nil {
		return err
	}
	if err := ensureMemberRole(client, *baseURL, token, superadminID, "superadmin", "global", "", ""); err != nil {
		return err
	}
	if err := ensureMemberRole(client, *baseURL, token, superadminID, "tenant_admin", "tenant", tenantID, ""); err != nil {
		return err
	}

	if err := ensureWorkspace(client, *baseURL, token, *tenantCode, *workspaceCode, *workspaceName); err != nil {
		return err
	}

	workspaceID, err := getWorkspaceID(client, *baseURL, token, *tenantCode, *workspaceCode)
	if err != nil {
		return err
	}

	assignments := []struct {
		Email       string
		Role        string
		ScopeType   string
		TenantID    string
		WorkspaceID string
	}{
		{
			Email:     *superadminEmail,
			Role:      "superadmin",
			ScopeType: "global",
		},
		// Superadmin also receives scoped roles in the seeded tenant/workspace so a
		// single OIDC token can exercise full management contract coverage.
		{
			Email:     *superadminEmail,
			Role:      "tenant_admin",
			ScopeType: "tenant",
			TenantID:  tenantID,
		},
		{
			Email:       *superadminEmail,
			Role:        "workspace_admin",
			ScopeType:   "workspace",
			TenantID:    tenantID,
			WorkspaceID: workspaceID,
		},
		{
			Email:       *superadminEmail,
			Role:        "workspace_editor",
			ScopeType:   "workspace",
			TenantID:    tenantID,
			WorkspaceID: workspaceID,
		},
		{
			Email:       *superadminEmail,
			Role:        "workspace_viewer",
			ScopeType:   "workspace",
			TenantID:    tenantID,
			WorkspaceID: workspaceID,
		},
		{
			Email:     *tenantAdminEmail,
			Role:      "tenant_admin",
			ScopeType: "tenant",
			TenantID:  tenantID,
		},
		{
			Email:       *workspaceAdminEmail,
			Role:        "workspace_admin",
			ScopeType:   "workspace",
			TenantID:    tenantID,
			WorkspaceID: workspaceID,
		},
		{
			Email:       *workspaceEditorEmail,
			Role:        "workspace_editor",
			ScopeType:   "workspace",
			TenantID:    tenantID,
			WorkspaceID: workspaceID,
		},
		{
			Email:       *workspaceViewerEmail,
			Role:        "workspace_viewer",
			ScopeType:   "workspace",
			TenantID:    tenantID,
			WorkspaceID: workspaceID,
		},
	}

	memberIDs := make(map[string]string, len(assignments))
	for _, a := range assignments {
		memberID, err := ensureMemberByEmail(client, *baseURL, token, a.Email)
		if err != nil {
			return err
		}
		memberIDs[a.Email] = memberID
		if err := ensureMemberRole(client, *baseURL, token, memberID, a.Role, a.ScopeType, a.TenantID, a.WorkspaceID); err != nil {
			return err
		}
	}

	noMemberState := "not-present"
	existingNoMemberID, err := findMemberByEmail(client, *baseURL, token, *noMemberEmail)
	if err != nil {
		return err
	}
	if existingNoMemberID != "" {
		noMemberState = "present-without-role-check"
	}

	fmt.Printf("rbac seeded: tenant=%s workspace=%s superadmin=%s tenant-admin=%s workspace-admin=%s workspace-editor=%s workspace-viewer=%s no-member=%s\n",
		*tenantCode, *workspaceCode,
		memberIDs[*superadminEmail],
		memberIDs[*tenantAdminEmail],
		memberIDs[*workspaceAdminEmail],
		memberIDs[*workspaceEditorEmail],
		memberIDs[*workspaceViewerEmail],
		noMemberState,
	)
	return nil
}

func cmdVisualDiff(args []string) error {
	fs := flag.NewFlagSet("visual-diff", flag.ContinueOnError)
	actualDir := fs.String("actual-dir", "", "directory with actual screenshots")
	goldenDir := fs.String("golden-dir", "test/system/baselines/golden", "directory with golden screenshots")
	pencilDir := fs.String("pencil-dir", "test/system/baselines/pencil", "directory with pencil screenshots")
	baselineMapPath := fs.String("baseline-map", "test/system/visual-baseline-map.json", "baseline map JSON")
	locales := fs.String("locales", "en,es", "comma-separated locales")
	viewports := fs.String("viewports", "desktop,mobile", "comma-separated viewports")
	criticalThreshold := fs.Float64("critical-threshold", 0.5, "critical diff threshold (percentage)")
	defaultThreshold := fs.Float64("default-threshold", 1.5, "default diff threshold (percentage)")
	allowMissing := fs.Bool("allow-missing-baselines", false, "do not fail when baseline files are missing")
	outHTML := fs.String("out-html", "", "output HTML report")
	outJSON := fs.String("out-json", "", "output JSON report")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *actualDir == "" {
		return errors.New("--actual-dir is required")
	}
	if *outHTML == "" || *outJSON == "" {
		return errors.New("--out-html and --out-json are required")
	}

	baselineMap, err := loadBaselineMap(*baselineMapPath)
	if err != nil {
		return err
	}

	locs := splitNonEmpty(*locales)
	vps := splitNonEmpty(*viewports)
	if len(locs) == 0 || len(vps) == 0 {
		return errors.New("locales/viewports must be non-empty")
	}

	var results []VisualDiffResult
	for _, entry := range baselineMap.Entries {
		for _, locale := range locs {
			for _, viewport := range vps {
				slug := routeSlug(entry.Route)
				name := fmt.Sprintf("%s.%s.%s.png", slug, viewport, locale)
				actualPath := filepath.Join(*actualDir, name)
				goldenPath := filepath.Join(*goldenDir, name)
				pencilPath := filepath.Join(*pencilDir, name)
				threshold := *defaultThreshold
				if entry.Critical {
					threshold = *criticalThreshold
				}

				res := VisualDiffResult{
					Route:        entry.Route,
					Viewport:     viewport,
					Locale:       locale,
					Critical:     entry.Critical,
					ThresholdPct: threshold,
					ActualPath:   actualPath,
					GoldenPath:   goldenPath,
					PencilPath:   pencilPath,
					Status:       "pass",
				}

				if _, err := os.Stat(actualPath); err != nil {
					res.Status = "fail"
					res.FailureMessage = fmt.Sprintf("actual screenshot missing: %s", actualPath)
					results = append(results, res)
					continue
				}
				if _, err := os.Stat(goldenPath); err != nil {
					if *allowMissing {
						res.Status = "skip"
						res.FailureMessage = fmt.Sprintf("golden baseline missing: %s", goldenPath)
						results = append(results, res)
						continue
					}
					res.Status = "fail"
					res.FailureMessage = fmt.Sprintf("golden baseline missing: %s", goldenPath)
					results = append(results, res)
					continue
				}
				if _, err := os.Stat(pencilPath); err != nil {
					if *allowMissing {
						res.Status = "skip"
						res.FailureMessage = fmt.Sprintf("pencil baseline missing: %s", pencilPath)
						results = append(results, res)
						continue
					}
					res.Status = "fail"
					res.FailureMessage = fmt.Sprintf("pencil baseline missing: %s", pencilPath)
					results = append(results, res)
					continue
				}

				goldenDiff, err := imageDiffPercent(actualPath, goldenPath)
				if err != nil {
					res.Status = "fail"
					res.FailureMessage = err.Error()
					results = append(results, res)
					continue
				}
				pencilDiff, err := imageDiffPercent(actualPath, pencilPath)
				if err != nil {
					res.Status = "fail"
					res.FailureMessage = err.Error()
					results = append(results, res)
					continue
				}

				res.GoldenDiffPct = goldenDiff
				res.PencilDiffPct = pencilDiff
				if goldenDiff > threshold || pencilDiff > threshold {
					res.Status = "fail"
					res.FailureMessage = fmt.Sprintf("diff exceeds threshold %.2f%% (golden %.3f%%, pencil %.3f%%)", threshold, goldenDiff, pencilDiff)
				}
				results = append(results, res)
			}
		}
	}

	if err := os.MkdirAll(filepath.Dir(*outJSON), 0o755); err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(*outHTML), 0o755); err != nil {
		return err
	}

	jsonBytes, err := json.MarshalIndent(results, "", "  ")
	if err != nil {
		return err
	}
	if err := os.WriteFile(*outJSON, jsonBytes, 0o644); err != nil {
		return err
	}
	if err := os.WriteFile(*outHTML, []byte(renderVisualHTML(results)), 0o644); err != nil {
		return err
	}

	var failed int
	for _, r := range results {
		if r.Status == "fail" {
			failed++
		}
	}
	fmt.Printf("visual diff completed: total=%d failed=%d\n", len(results), failed)
	if failed > 0 {
		return fmt.Errorf("visual diff failed: %d entries", failed)
	}
	return nil
}

func cmdJunit(args []string) error {
	fs := flag.NewFlagSet("junit", flag.ContinueOnError)
	resultsPath := fs.String("results", "", "stage results TSV")
	suite := fs.String("suite", "system-tests", "suite name")
	out := fs.String("out", "", "output XML file")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *resultsPath == "" || *out == "" {
		return errors.New("--results and --out are required")
	}

	results, err := readStageTSV(*resultsPath)
	if err != nil {
		return err
	}
	xmlBytes, err := renderJUnitXML(*suite, results)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(*out), 0o755); err != nil {
		return err
	}
	if err := os.WriteFile(*out, xmlBytes, 0o644); err != nil {
		return err
	}
	fmt.Printf("junit written: %s\n", *out)
	return nil
}

func cmdRunResult(args []string) error {
	fs := flag.NewFlagSet("run-result", flag.ContinueOnError)
	resultsPath := fs.String("results", "", "stage results TSV")
	mode := fs.String("mode", "pr", "pr or nightly")
	artifactDir := fs.String("artifact-dir", "", "artifact directory")
	out := fs.String("out", "", "output JSON path")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *resultsPath == "" || *out == "" {
		return errors.New("--results and --out are required")
	}

	results, err := readStageTSV(*resultsPath)
	if err != nil {
		return err
	}

	summary := map[string]int{"pass": 0, "fail": 0, "skip": 0}
	for _, r := range results {
		summary[r.Status]++
	}
	payload := map[string]any{
		"timestamp":    time.Now().UTC().Format(time.RFC3339),
		"mode":         *mode,
		"artifact_dir": *artifactDir,
		"summary":      summary,
		"stages":       results,
		"exit_code":    boolToExitCode(summary["fail"] > 0),
	}
	content, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(*out), 0o755); err != nil {
		return err
	}
	if err := os.WriteFile(*out, content, 0o644); err != nil {
		return err
	}
	fmt.Printf("run result written: %s\n", *out)
	return nil
}

func scanRoutes(appDir string) ([]RouteInfo, error) {
	var routes []RouteInfo
	seen := map[string]struct{}{}

	err := filepath.WalkDir(appDir, func(path string, d os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if d.IsDir() {
			return nil
		}
		if filepath.Base(path) != "page.tsx" {
			return nil
		}
		route, err := routeFromPath(appDir, path)
		if err != nil {
			return err
		}
		if _, ok := seen[route]; ok {
			return nil
		}
		seen[route] = struct{}{}
		routes = append(routes, RouteInfo{
			Route:   route,
			Dynamic: strings.Contains(route, "[") && strings.Contains(route, "]"),
			Scope:   routeScope(route),
			Family:  routeFamily(route),
		})
		return nil
	})
	if err != nil {
		return nil, err
	}

	sort.Slice(routes, func(i, j int) bool {
		return routes[i].Route < routes[j].Route
	})
	return routes, nil
}

func routeFromPath(appDir, path string) (string, error) {
	rel, err := filepath.Rel(appDir, path)
	if err != nil {
		return "", err
	}
	rel = filepath.ToSlash(rel)
	dir := strings.TrimSuffix(rel, "/page.tsx")
	if dir == "page.tsx" || dir == "." || dir == "" {
		return "/", nil
	}

	parts := strings.Split(dir, "/")
	clean := make([]string, 0, len(parts))
	for _, p := range parts {
		if p == "" {
			continue
		}
		if strings.HasPrefix(p, "(") && strings.HasSuffix(p, ")") {
			continue
		}
		clean = append(clean, p)
	}
	if len(clean) == 0 {
		return "/", nil
	}
	return "/" + strings.Join(clean, "/"), nil
}

func routeScope(route string) string {
	switch {
	case strings.HasPrefix(route, "/global"):
		return "global"
	case strings.HasPrefix(route, "/t/[tenantCode]/w/[workspaceCode]"):
		return "workspace"
	case strings.HasPrefix(route, "/t/[tenantCode]"):
		return "tenant"
	default:
		return "public"
	}
}

func routeFamily(route string) string {
	switch {
	case route == "/":
		return "root"
	case route == "/login":
		return "auth-login"
	case route == "/onboarding":
		return "auth-onboarding"
	case route == "/access-denied":
		return "auth-access-denied"
	case strings.HasPrefix(route, "/global"):
		return "dashboard-global"
	case strings.HasPrefix(route, "/t/[tenantCode]/w/[workspaceCode]"):
		return "dashboard-workspace"
	case strings.HasPrefix(route, "/t/[tenantCode]"):
		return "dashboard-tenant"
	default:
		return "unknown"
	}
}

func loadManifest(path string) (ScreenManifest, error) {
	var m ScreenManifest
	b, err := os.ReadFile(path)
	if err != nil {
		return m, err
	}
	if err := json.Unmarshal(b, &m); err != nil {
		return m, err
	}
	return m, nil
}

func loadBaselineMap(path string) (VisualBaselineMap, error) {
	var m VisualBaselineMap
	b, err := os.ReadFile(path)
	if err != nil {
		return m, err
	}
	if err := json.Unmarshal(b, &m); err != nil {
		return m, err
	}
	return m, nil
}

func effectiveList(primary, defaults []string) []string {
	if len(primary) > 0 {
		return primary
	}
	return defaults
}

func expandMatrix(m ScreenManifest) []MatrixRow {
	rows := make([]MatrixRow, 0)
	for _, s := range m.Screens {
		roles := effectiveList(s.Role, m.MatrixDefaults.Role)
		locales := effectiveList(s.Locale, m.MatrixDefaults.Locale)
		viewports := effectiveList(s.Viewport, m.MatrixDefaults.Viewport)
		for _, scope := range s.Scope {
			for _, role := range roles {
				for _, locale := range locales {
					for _, viewport := range viewports {
						rows = append(rows, MatrixRow{
							Route:         s.Route,
							RouteSlug:     routeSlug(s.Route),
							Scope:         scope,
							Role:          role,
							Locale:        locale,
							Viewport:      viewport,
							Critical:      s.Critical,
							PencilFrameID: s.PencilFrameID,
							Preconditions: strings.Join(s.Preconditions, " | "),
							Actions:       strings.Join(s.Actions, " | "),
							Assertions:    strings.Join(s.Assertions, " | "),
						})
					}
				}
			}
		}
	}

	sort.Slice(rows, func(i, j int) bool {
		if rows[i].Route != rows[j].Route {
			return rows[i].Route < rows[j].Route
		}
		if rows[i].Locale != rows[j].Locale {
			return rows[i].Locale < rows[j].Locale
		}
		if rows[i].Viewport != rows[j].Viewport {
			return rows[i].Viewport < rows[j].Viewport
		}
		if rows[i].Scope != rows[j].Scope {
			return rows[i].Scope < rows[j].Scope
		}
		return rows[i].Role < rows[j].Role
	})
	return rows
}

func routeSlug(route string) string {
	if route == "/" {
		return "root"
	}
	s := strings.TrimPrefix(route, "/")
	s = strings.ReplaceAll(s, "/", "__")
	s = strings.ReplaceAll(s, "[", "")
	s = strings.ReplaceAll(s, "]", "")
	s = strings.ReplaceAll(s, ":", "_")
	return s
}

func splitNonEmpty(s string) []string {
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}

func imageDiffPercent(actualPath, baselinePath string) (float64, error) {
	actual, err := loadImage(actualPath)
	if err != nil {
		return 0, fmt.Errorf("decode actual %s: %w", actualPath, err)
	}
	baseline, err := loadImage(baselinePath)
	if err != nil {
		return 0, fmt.Errorf("decode baseline %s: %w", baselinePath, err)
	}

	aBounds := actual.Bounds()
	bBounds := baseline.Bounds()
	if !aBounds.Eq(bBounds) {
		return 100, nil
	}

	threshold := uint32(24)
	var diffCount int64
	total := int64(aBounds.Dx() * aBounds.Dy())
	for y := aBounds.Min.Y; y < aBounds.Max.Y; y++ {
		for x := aBounds.Min.X; x < aBounds.Max.X; x++ {
			ar, ag, ab, aa := actual.At(x, y).RGBA()
			br, bg, bb, ba := baseline.At(x, y).RGBA()
			delta := absDiff(ar, br) + absDiff(ag, bg) + absDiff(ab, bb) + absDiff(aa, ba)
			if delta > threshold {
				diffCount++
			}
		}
	}
	if total == 0 {
		return 0, nil
	}
	pct := (float64(diffCount) / float64(total)) * 100
	return round3(pct), nil
}

func absDiff(a, b uint32) uint32 {
	if a > b {
		return a - b
	}
	return b - a
}

func closeWithError(closer io.Closer, errp *error) {
	if cerr := closer.Close(); cerr != nil && *errp == nil {
		*errp = cerr
	}
}

func flushCSVWithError(w *csv.Writer, errp *error) {
	w.Flush()
	if cerr := w.Error(); cerr != nil && *errp == nil {
		*errp = cerr
	}
}

func round3(v float64) float64 {
	return math.Round(v*1000) / 1000
}

func loadImage(path string) (_ image.Image, err error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer closeWithError(f, &err)
	img, _, err := image.Decode(f)
	if err != nil {
		return nil, err
	}
	return img, nil
}

func renderVisualHTML(results []VisualDiffResult) string {
	var b strings.Builder
	b.WriteString("<!doctype html><html><head><meta charset=\"utf-8\"><title>Visual Diff Report</title>")
	b.WriteString("<style>body{font-family:Arial,sans-serif;padding:16px}table{border-collapse:collapse;width:100%}th,td{border:1px solid #ccc;padding:8px;font-size:12px}th{background:#f4f4f4}.pass{background:#e8f8e8}.fail{background:#fde8e8}.skip{background:#fff8e1}</style>")
	b.WriteString("</head><body>")
	b.WriteString("<h1>Visual Diff Report</h1>")
	b.WriteString("<table><thead><tr><th>Route</th><th>Viewport</th><th>Locale</th><th>Critical</th><th>Golden %</th><th>Pencil %</th><th>Threshold %</th><th>Status</th><th>Details</th></tr></thead><tbody>")
	for _, r := range results {
		b.WriteString("<tr class=\"")
		b.WriteString(htmlEscape(r.Status))
		b.WriteString("\"><td>")
		b.WriteString(htmlEscape(r.Route))
		b.WriteString("</td><td>")
		b.WriteString(htmlEscape(r.Viewport))
		b.WriteString("</td><td>")
		b.WriteString(htmlEscape(r.Locale))
		b.WriteString("</td><td>")
		b.WriteString(strconv.FormatBool(r.Critical))
		b.WriteString("</td><td>")
		b.WriteString(strconv.FormatFloat(r.GoldenDiffPct, 'f', 3, 64))
		b.WriteString("</td><td>")
		b.WriteString(strconv.FormatFloat(r.PencilDiffPct, 'f', 3, 64))
		b.WriteString("</td><td>")
		b.WriteString(strconv.FormatFloat(r.ThresholdPct, 'f', 2, 64))
		b.WriteString("</td><td>")
		b.WriteString(htmlEscape(strings.ToUpper(r.Status)))
		b.WriteString("</td><td>")
		b.WriteString(htmlEscape(r.FailureMessage))
		b.WriteString("</td></tr>")
	}
	b.WriteString("</tbody></table></body></html>")
	return b.String()
}

func htmlEscape(s string) string {
	r := strings.NewReplacer(
		"&", "&amp;",
		"<", "&lt;",
		">", "&gt;",
		"\"", "&quot;",
		"'", "&#39;",
	)
	return r.Replace(s)
}

func readStageTSV(path string) (_ []stageResult, err error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer closeWithError(f, &err)

	results := make([]stageResult, 0)
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		parts := strings.SplitN(line, "\t", 4)
		if len(parts) < 4 {
			return nil, fmt.Errorf("invalid stage result line: %q", line)
		}
		dur, err := strconv.ParseInt(parts[2], 10, 64)
		if err != nil {
			return nil, fmt.Errorf("invalid duration in line %q: %w", line, err)
		}
		results = append(results, stageResult{
			Name:       parts[0],
			Status:     parts[1],
			DurationMs: dur,
			LogPath:    parts[3],
		})
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return results, nil
}

type xmlFailure struct {
	Message string `xml:"message,attr"`
	Body    string `xml:",chardata"`
}

type xmlTestCase struct {
	XMLName   xml.Name    `xml:"testcase"`
	Name      string      `xml:"name,attr"`
	ClassName string      `xml:"classname,attr"`
	Time      string      `xml:"time,attr"`
	Failure   *xmlFailure `xml:"failure,omitempty"`
}

type xmlSuite struct {
	XMLName  xml.Name      `xml:"testsuite"`
	Name     string        `xml:"name,attr"`
	Tests    int           `xml:"tests,attr"`
	Failures int           `xml:"failures,attr"`
	Time     string        `xml:"time,attr"`
	Cases    []xmlTestCase `xml:"testcase"`
}

func renderJUnitXML(suiteName string, results []stageResult) ([]byte, error) {
	cases := make([]xmlTestCase, 0, len(results))
	failures := 0
	var totalMs int64
	for _, r := range results {
		totalMs += r.DurationMs
		tc := xmlTestCase{
			Name:      r.Name,
			ClassName: suiteName,
			Time:      fmt.Sprintf("%.3f", float64(r.DurationMs)/1000),
		}
		if r.Status == "fail" {
			failures++
			tc.Failure = &xmlFailure{
				Message: "stage failed",
				Body:    "See log: " + r.LogPath,
			}
		}
		cases = append(cases, tc)
	}

	suite := xmlSuite{
		Name:     suiteName,
		Tests:    len(results),
		Failures: failures,
		Time:     fmt.Sprintf("%.3f", float64(totalMs)/1000),
		Cases:    cases,
	}

	out, err := xml.MarshalIndent(suite, "", "  ")
	if err != nil {
		return nil, err
	}
	out = append([]byte(xml.Header), out...)
	return out, nil
}

func ensureOnboarding(client *http.Client, baseURL, token, tenantCode, tenantName string) error {
	payload := map[string]any{
		"tenant_code": tenantCode,
		"tenant_name": tenantName,
	}
	status, body, err := doJSON(client, http.MethodPost, strings.TrimSuffix(baseURL, "/")+"/api/v1/onboarding/setup", "Bearer "+token, payload)
	if err != nil {
		return err
	}
	if status != http.StatusCreated && status != http.StatusConflict {
		return fmt.Errorf("onboarding setup failed: status=%d body=%s", status, string(body))
	}
	return nil
}

func ensureWorkspace(client *http.Client, baseURL, token, tenantCode, workspaceCode, workspaceName string) error {
	path := fmt.Sprintf("%s/api/v1/manage/tenants/%s/workspaces", strings.TrimSuffix(baseURL, "/"), tenantCode)
	payload := map[string]any{
		"code": workspaceCode,
		"name": workspaceName,
	}
	status, body, err := doJSON(client, http.MethodPost, path, "Bearer "+token, payload)
	if err != nil {
		return err
	}
	if status != http.StatusCreated && status != http.StatusConflict {
		return fmt.Errorf("workspace ensure failed: status=%d body=%s", status, string(body))
	}
	return nil
}

func createAPIKey(client *http.Client, baseURL, token, tenantCode, workspaceCode string) (string, error) {
	path := fmt.Sprintf("%s/api/v1/manage/tenants/%s/workspaces/%s/api-keys", strings.TrimSuffix(baseURL, "/"), tenantCode, workspaceCode)
	payload := map[string]any{
		"name": fmt.Sprintf("system-test-%d", time.Now().UnixNano()),
	}
	status, body, err := doJSON(client, http.MethodPost, path, "Bearer "+token, payload)
	if err != nil {
		return "", err
	}
	if status != http.StatusCreated {
		return "", fmt.Errorf("create api key failed: status=%d body=%s", status, string(body))
	}
	var resp struct {
		Key   string `json:"key"`
		Token string `json:"token"`
	}
	if err := json.Unmarshal(body, &resp); err != nil {
		return "", err
	}
	apiKey := strings.TrimSpace(resp.Key)
	if apiKey == "" {
		apiKey = strings.TrimSpace(resp.Token)
	}
	if apiKey == "" {
		return "", errors.New("api key response missing key/token")
	}
	return apiKey, nil
}

func latestTrackingID(client *http.Client, baseURL, token, tenantCode, workspaceCode string) (string, error) {
	path := fmt.Sprintf("%s/api/v1/manage/tenants/%s/workspaces/%s/emails?limit=1", strings.TrimSuffix(baseURL, "/"), tenantCode, workspaceCode)
	status, body, err := doJSON(client, http.MethodGet, path, "Bearer "+token, nil)
	if err != nil {
		return "", err
	}
	if status != http.StatusOK {
		return "", fmt.Errorf("list emails failed: status=%d body=%s", status, string(body))
	}
	var resp struct {
		Items []struct {
			TrackingID string `json:"tracking_id"`
		} `json:"items"`
	}
	if err := json.Unmarshal(body, &resp); err != nil {
		return "", err
	}
	if len(resp.Items) == 0 {
		return "", nil
	}
	return resp.Items[0].TrackingID, nil
}

func sendOneEmail(client *http.Client, baseURL, apiKey, tenantCode, workspaceCode, templateType, recipient string) (string, error) {
	path := strings.TrimSuffix(baseURL, "/") + "/api/v1/send"
	payload := map[string]any{
		"ref": fmt.Sprintf("%s:%s:%s", tenantCode, workspaceCode, templateType),
		"to":  []string{recipient},
		"variables": map[string]any{
			"first_name":   "QA",
			"company_name": "Senda",
		},
	}
	status, body, err := doJSON(client, http.MethodPost, path, "Bearer "+apiKey, payload)
	if err != nil {
		return "", err
	}
	if status != http.StatusAccepted {
		return "", fmt.Errorf("send email failed: status=%d body=%s", status, string(body))
	}
	var resp struct {
		TrackingIDs []struct {
			TrackingID string `json:"tracking_id"`
		} `json:"tracking_ids"`
	}
	if err := json.Unmarshal(body, &resp); err != nil {
		return "", err
	}
	if len(resp.TrackingIDs) == 0 || resp.TrackingIDs[0].TrackingID == "" {
		return "", errors.New("send response missing tracking_id")
	}
	return resp.TrackingIDs[0].TrackingID, nil
}

func doJSON(client *http.Client, method, endpoint, authHeader string, payload any) (_ int, _ []byte, err error) {
	var body io.Reader
	if payload != nil {
		b, err := json.Marshal(payload)
		if err != nil {
			return 0, nil, err
		}
		body = bytes.NewReader(b)
	}
	req, err := http.NewRequest(method, endpoint, body)
	if err != nil {
		return 0, nil, err
	}
	if payload != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if authHeader != "" {
		req.Header.Set("Authorization", authHeader)
	}
	resp, err := client.Do(req)
	if err != nil {
		return 0, nil, err
	}
	defer closeWithError(resp.Body, &err)
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return resp.StatusCode, nil, err
	}
	return resp.StatusCode, data, nil
}

func keycloakAdminToken(baseURL, adminUser, adminPass string) (_ string, err error) {
	endpoint := strings.TrimSuffix(baseURL, "/") + "/realms/master/protocol/openid-connect/token"
	form := url.Values{}
	form.Set("grant_type", "password")
	form.Set("client_id", "admin-cli")
	form.Set("username", adminUser)
	form.Set("password", adminPass)

	req, err := http.NewRequest(http.MethodPost, endpoint, strings.NewReader(form.Encode()))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	client := &http.Client{Timeout: 20 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer closeWithError(resp.Body, &err)
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("keycloak admin token failed: status=%d body=%s", resp.StatusCode, string(body))
	}
	var out struct {
		AccessToken string `json:"access_token"`
	}
	if err := json.Unmarshal(body, &out); err != nil {
		return "", err
	}
	if out.AccessToken == "" {
		return "", errors.New("keycloak admin token response missing access_token")
	}
	return out.AccessToken, nil
}

func keycloakUserExists(baseURL, realm, adminToken, email string) (_ bool, err error) {
	endpoint := fmt.Sprintf("%s/admin/realms/%s/users?username=%s", strings.TrimSuffix(baseURL, "/"), realm, url.QueryEscape(email))
	req, err := http.NewRequest(http.MethodGet, endpoint, nil)
	if err != nil {
		return false, err
	}
	req.Header.Set("Authorization", "Bearer "+adminToken)
	client := &http.Client{Timeout: 20 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return false, err
	}
	defer closeWithError(resp.Body, &err)
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return false, err
	}
	if resp.StatusCode != http.StatusOK {
		return false, fmt.Errorf("keycloak list users failed: status=%d body=%s", resp.StatusCode, string(body))
	}
	var users []map[string]any
	if err := json.Unmarshal(body, &users); err != nil {
		return false, err
	}
	return len(users) > 0, nil
}

func keycloakCreateUser(baseURL, realm, adminToken, email, password string) (err error) {
	endpoint := fmt.Sprintf("%s/admin/realms/%s/users", strings.TrimSuffix(baseURL, "/"), realm)
	payload := map[string]any{
		"username":      email,
		"email":         email,
		"emailVerified": true,
		"enabled":       true,
		"firstName":     "System",
		"lastName":      "User",
		"credentials": []map[string]any{
			{
				"type":      "password",
				"value":     password,
				"temporary": false,
			},
		},
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	req, err := http.NewRequest(http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+adminToken)
	client := &http.Client{Timeout: 20 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer closeWithError(resp.Body, &err)
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	if resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusConflict {
		return fmt.Errorf("keycloak create user failed: status=%d body=%s", resp.StatusCode, string(respBody))
	}
	return nil
}

func getTenantID(client *http.Client, baseURL, token, tenantCode string) (string, error) {
	path := fmt.Sprintf("%s/api/v1/manage/tenants/%s", strings.TrimSuffix(baseURL, "/"), tenantCode)
	status, body, err := doJSON(client, http.MethodGet, path, "Bearer "+token, nil)
	if err != nil {
		return "", err
	}
	if status == http.StatusOK {
		var resp struct {
			ID string `json:"id"`
		}
		if err := json.Unmarshal(body, &resp); err != nil {
			return "", err
		}
		if strings.TrimSpace(resp.ID) != "" {
			return resp.ID, nil
		}
	}

	// Fallback for callers that only have superadmin role (no tenant_admin on target tenant).
	listPath := fmt.Sprintf("%s/api/v1/manage/tenants?limit=200", strings.TrimSuffix(baseURL, "/"))
	status, body, err = doJSON(client, http.MethodGet, listPath, "Bearer "+token, nil)
	if err != nil {
		return "", err
	}
	if status != http.StatusOK {
		return "", fmt.Errorf("list tenants failed while resolving tenant id: status=%d body=%s", status, string(body))
	}

	var resp struct {
		Items []struct {
			ID   string `json:"id"`
			Code string `json:"code"`
		} `json:"items"`
	}
	if err := json.Unmarshal(body, &resp); err != nil {
		return "", err
	}
	for _, item := range resp.Items {
		if strings.EqualFold(strings.TrimSpace(item.Code), strings.TrimSpace(tenantCode)) && strings.TrimSpace(item.ID) != "" {
			return item.ID, nil
		}
	}
	return "", fmt.Errorf("tenant %q not found", tenantCode)
}

func getWorkspaceID(client *http.Client, baseURL, token, tenantCode, workspaceCode string) (string, error) {
	path := fmt.Sprintf("%s/api/v1/manage/tenants/%s/workspaces/%s", strings.TrimSuffix(baseURL, "/"), tenantCode, workspaceCode)
	status, body, err := doJSON(client, http.MethodGet, path, "Bearer "+token, nil)
	if err != nil {
		return "", err
	}
	if status == http.StatusOK {
		var resp struct {
			ID string `json:"id"`
		}
		if err := json.Unmarshal(body, &resp); err != nil {
			return "", err
		}
		if strings.TrimSpace(resp.ID) != "" {
			return resp.ID, nil
		}
	}

	// Fallback for caller roles that cannot access workspace detail but can list by tenant.
	listPath := fmt.Sprintf("%s/api/v1/manage/tenants/%s/workspaces?limit=200", strings.TrimSuffix(baseURL, "/"), tenantCode)
	status, body, err = doJSON(client, http.MethodGet, listPath, "Bearer "+token, nil)
	if err != nil {
		return "", err
	}
	if status != http.StatusOK {
		return "", fmt.Errorf("list workspaces failed while resolving workspace id: status=%d body=%s", status, string(body))
	}

	var resp struct {
		Items []struct {
			ID   string `json:"id"`
			Code string `json:"code"`
		} `json:"items"`
	}
	if err := json.Unmarshal(body, &resp); err != nil {
		return "", err
	}
	for _, item := range resp.Items {
		if strings.EqualFold(strings.TrimSpace(item.Code), strings.TrimSpace(workspaceCode)) && strings.TrimSpace(item.ID) != "" {
			return item.ID, nil
		}
	}
	return "", fmt.Errorf("workspace %q not found in tenant %q", workspaceCode, tenantCode)
}

func ensureMemberByEmail(client *http.Client, baseURL, token, email string) (string, error) {
	existing, err := findMemberByEmail(client, baseURL, token, email)
	if err != nil {
		return "", err
	}
	if existing != "" {
		return existing, nil
	}

	path := fmt.Sprintf("%s/api/v1/manage/members", strings.TrimSuffix(baseURL, "/"))
	payload := map[string]any{"email": email}
	status, body, err := doJSON(client, http.MethodPost, path, "Bearer "+token, payload)
	if err != nil {
		return "", err
	}
	if status != http.StatusCreated && status != http.StatusConflict {
		return "", fmt.Errorf("create member failed: status=%d body=%s", status, string(body))
	}
	if status == http.StatusCreated {
		var resp struct {
			ID string `json:"id"`
		}
		if err := json.Unmarshal(body, &resp); err != nil {
			return "", err
		}
		if strings.TrimSpace(resp.ID) != "" {
			return resp.ID, nil
		}
	}

	created, err := findMemberByEmail(client, baseURL, token, email)
	if err != nil {
		return "", err
	}
	if created == "" {
		return "", fmt.Errorf("member not found after create: %s", email)
	}
	return created, nil
}

func findMemberByEmail(client *http.Client, baseURL, token, email string) (string, error) {
	path := fmt.Sprintf("%s/api/v1/manage/members?limit=200", strings.TrimSuffix(baseURL, "/"))
	status, body, err := doJSON(client, http.MethodGet, path, "Bearer "+token, nil)
	if err != nil {
		return "", err
	}
	if status != http.StatusOK {
		return "", fmt.Errorf("list members failed: status=%d body=%s", status, string(body))
	}

	var resp struct {
		Items []struct {
			ID    string `json:"id"`
			Email string `json:"email"`
		} `json:"items"`
	}
	if err := json.Unmarshal(body, &resp); err != nil {
		return "", err
	}
	for _, item := range resp.Items {
		if strings.EqualFold(strings.TrimSpace(item.Email), strings.TrimSpace(email)) {
			return item.ID, nil
		}
	}
	return "", nil
}

func ensureMemberRole(client *http.Client, baseURL, token, memberID, role, scopeType, tenantID, workspaceID string) error {
	path := fmt.Sprintf("%s/api/v1/manage/members/%s/roles", strings.TrimSuffix(baseURL, "/"), memberID)
	payload := map[string]any{
		"role":       role,
		"scope_type": scopeType,
	}
	if strings.TrimSpace(tenantID) != "" {
		payload["tenant_id"] = tenantID
	}
	if strings.TrimSpace(workspaceID) != "" {
		payload["workspace_id"] = workspaceID
	}

	status, body, err := doJSON(client, http.MethodPost, path, "Bearer "+token, payload)
	if err != nil {
		return err
	}
	if status != http.StatusCreated && status != http.StatusConflict {
		return fmt.Errorf("add role failed: status=%d body=%s", status, string(body))
	}
	return nil
}

func envOrDefault(key, fallback string) string {
	if v := strings.TrimSpace(os.Getenv(key)); v != "" {
		return v
	}
	return fallback
}

func writeOutput(path string, content []byte) error {
	if path == "" {
		_, err := os.Stdout.Write(content)
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, content, 0o644)
}

func boolToExitCode(failed bool) int {
	if failed {
		return 1
	}
	return 0
}
