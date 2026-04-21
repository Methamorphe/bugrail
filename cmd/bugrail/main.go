package main

import (
	"bufio"
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"strings"

	"golang.org/x/term"

	"github.com/Methamorphe/bugrail/internal/auth"
	"github.com/Methamorphe/bugrail/internal/config"
	"github.com/Methamorphe/bugrail/internal/server"
	"github.com/Methamorphe/bugrail/internal/storage"
)

func main() {
	os.Exit(run(context.Background(), os.Args[1:]))
}

func run(ctx context.Context, args []string) int {
	if len(args) == 0 {
		fmt.Fprintln(os.Stderr, "usage: bugrail <init|serve|migrate>")
		return 2
	}

	cfg := config.Load()
	logger := newLogger()

	var err error
	switch args[0] {
	case "init":
		err = runInit(ctx, cfg, logger, args[1:])
	case "serve":
		err = runServe(ctx, cfg, logger)
	case "migrate":
		err = runMigrate(ctx, cfg)
	default:
		fmt.Fprintf(os.Stderr, "unknown command %q\n", args[0])
		return 2
	}

	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	return 0
}

func runInit(ctx context.Context, cfg config.Config, logger *slog.Logger, args []string) error {
	fs := flag.NewFlagSet("init", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)

	var adminEmail string
	var adminPassword string
	var orgName string
	var projectName string
	fs.StringVar(&adminEmail, "admin-email", "", "admin email")
	fs.StringVar(&adminPassword, "admin-password", "", "admin password")
	fs.StringVar(&orgName, "org-name", "", "organization name")
	fs.StringVar(&projectName, "project-name", "", "project name")
	if err := fs.Parse(args); err != nil {
		return err
	}

	isTTY := term.IsTerminal(int(os.Stdin.Fd()))
	var err error
	if adminEmail == "" {
		adminEmail, err = promptString("Admin email", isTTY)
		if err != nil {
			return err
		}
	}
	if adminPassword == "" {
		adminPassword, err = promptPassword("Admin password", isTTY)
		if err != nil {
			return err
		}
	}
	if orgName == "" {
		orgName, err = promptString("Organization name", isTTY)
		if err != nil {
			return err
		}
	}
	if projectName == "" {
		projectName, err = promptString("Project name", isTTY)
		if err != nil {
			return err
		}
	}

	passwordHash, err := auth.HashPassword(adminPassword)
	if err != nil {
		return err
	}

	store, err := storage.Open(ctx, cfg)
	if err != nil {
		return err
	}
	defer store.Close()

	if err := store.Migrate(ctx); err != nil {
		return err
	}

	result, err := store.Bootstrap(ctx, storage.BootstrapParams{
		AdminEmail:   strings.ToLower(strings.TrimSpace(adminEmail)),
		PasswordHash: passwordHash,
		OrgName:      strings.TrimSpace(orgName),
		OrgSlug:      slugify(orgName),
		ProjectName:  strings.TrimSpace(projectName),
		ProjectSlug:  slugify(projectName),
	})
	if err != nil {
		if errors.Is(err, storage.ErrAlreadyInitialized) {
			return fmt.Errorf("bugrail is already initialized")
		}
		return err
	}

	dsn, err := sentryDSN(cfg.BaseURL(), result.ProjectID, result.PublicKey)
	if err != nil {
		return err
	}

	logger.Info("bugrail initialized", "project_id", result.ProjectID)
	fmt.Printf("Admin: %s\n", strings.ToLower(strings.TrimSpace(adminEmail)))
	fmt.Printf("Project: %s\n", projectName)
	fmt.Printf("DSN: %s\n", dsn)
	return nil
}

func runServe(ctx context.Context, cfg config.Config, logger *slog.Logger) error {
	store, err := storage.Open(ctx, cfg)
	if err != nil {
		return err
	}
	defer store.Close()

	if err := store.Migrate(ctx); err != nil {
		return err
	}

	handler, err := server.New(store, logger, cfg.RateLimitPerProject, cfg.BaseURL())
	if err != nil {
		return err
	}

	logger.Info("starting bugrail", "listen_addr", cfg.ListenAddr, "driver", cfg.Driver())
	return http.ListenAndServe(cfg.ListenAddr, handler)
}

func runMigrate(ctx context.Context, cfg config.Config) error {
	store, err := storage.Open(ctx, cfg)
	if err != nil {
		return err
	}
	defer store.Close()
	return store.Migrate(ctx)
}

func newLogger() *slog.Logger {
	if term.IsTerminal(int(os.Stdout.Fd())) {
		return slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	}
	return slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
}

func promptString(label string, interactive bool) (string, error) {
	if !interactive {
		return "", fmt.Errorf("%s is required in non-interactive mode", strings.ToLower(label))
	}
	fmt.Printf("%s: ", label)
	reader := bufio.NewReader(os.Stdin)
	value, err := reader.ReadString('\n')
	if err != nil && !errors.Is(err, io.EOF) {
		return "", fmt.Errorf("read %s: %w", strings.ToLower(label), err)
	}
	value = strings.TrimSpace(value)
	if value == "" {
		return "", fmt.Errorf("%s cannot be empty", strings.ToLower(label))
	}
	return value, nil
}

func promptPassword(label string, interactive bool) (string, error) {
	if !interactive {
		return "", fmt.Errorf("%s is required in non-interactive mode", strings.ToLower(label))
	}
	fmt.Printf("%s: ", label)
	bytes, err := term.ReadPassword(int(os.Stdin.Fd()))
	fmt.Println()
	if err != nil {
		return "", fmt.Errorf("read %s: %w", strings.ToLower(label), err)
	}
	value := strings.TrimSpace(string(bytes))
	if value == "" {
		return "", fmt.Errorf("%s cannot be empty", strings.ToLower(label))
	}
	return value, nil
}

func sentryDSN(baseURL, projectID, publicKey string) (string, error) {
	u, err := url.Parse(baseURL)
	if err != nil {
		return "", fmt.Errorf("parse base url: %w", err)
	}
	u.User = url.User(publicKey)
	u.Path = "/" + projectID
	return u.String(), nil
}

func slugify(raw string) string {
	raw = strings.ToLower(strings.TrimSpace(raw))
	if raw == "" {
		return "default"
	}
	var b strings.Builder
	lastDash := false
	for _, r := range raw {
		switch {
		case r >= 'a' && r <= 'z':
			b.WriteRune(r)
			lastDash = false
		case r >= '0' && r <= '9':
			b.WriteRune(r)
			lastDash = false
		default:
			if !lastDash && b.Len() > 0 {
				b.WriteByte('-')
				lastDash = true
			}
		}
	}
	slug := strings.Trim(b.String(), "-")
	if slug == "" {
		return "default"
	}
	return slug
}
