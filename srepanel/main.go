package main

import (
	"context"
	"errors"
	"flag"
	"log"
	"net/http"
	"os"
	"os/signal"
	"time"

	"go.mkw.re/ghidra-panel/common"
	"go.mkw.re/ghidra-panel/ghidra"
	"golang.org/x/sync/errgroup"

	"go.mkw.re/ghidra-panel/database"
	"go.mkw.re/ghidra-panel/discord"
	"go.mkw.re/ghidra-panel/oauth"
	"go.mkw.re/ghidra-panel/token"
	"go.mkw.re/ghidra-panel/web"
)

func main() {
	// cli args
	if len(os.Args) > 1 {
		switch os.Args[1] {
		case "rename":
			os.Args = os.Args[1:]
			dbPath := flag.String("db", "ghidra_panel.db", "path to database file")
			argUserID := flag.Uint64("user-id", 0, "ID of user to rename")
			argUser := flag.String("user", "", "new username")
			flag.Parse()

			db, err := database.Open(*dbPath)
			if err != nil {
				log.Fatal(err)
			}
			defer db.Close()

			// Default to "discord" provider for backwards compatibility
			provider := "discord"
			if err := db.SetUsername(context.Background(), *argUserID, *argUser, provider); err != nil {
				log.Fatal(err)
			}
			return
		case "set-password":
			os.Args = os.Args[1:]
			dbPath := flag.String("db", "ghidra_panel.db", "path to database file")
			argUserID := flag.Uint64("user-id", 0, "user id to set password for")
			argUser := flag.String("user", "", "user to set password for")
			argPass := flag.String("pass", "", "password to set")
			flag.Parse()
			updateAccount(*dbPath, *argUserID, *argUser, *argPass)
			return
		}
	}

	// prod args
	configPath := flag.String("config", "ghidra_panel.yaml", "path to config file (YAML)")
	secretsPath := flag.String("secrets", "ghidra_panel.secrets.json", "path to secrets file")
	dbPath := flag.String("db", "ghidra_panel.db", "path to database file")
	listen := flag.String("listen", ":8080", "listen address")
	cmdInit := flag.Bool("init", false, "initialize database and exit")
	cmdClean := flag.Bool("clean", false, "remove database file and exit")
	dev := flag.Bool("dev", false, "enable development mode")

	flag.Parse()

	// Handle -clean flag
	if *cmdClean {
		if err := os.Remove(*dbPath); err != nil && !os.IsNotExist(err) {
			log.Fatal(err)
		}
		log.Printf("Removed database: %s\n", *dbPath)
		return
	}

	// Read config

	cfg, err := loadConfig(*configPath)
	if err != nil {
		if os.IsNotExist(err) {
			log.Fatalf("Config file not found: %s\n\nPlease copy config.example.yaml to %s and customize it with your OAuth credentials.\nSee README.md for setup instructions.", *configPath, *configPath)
		}
		log.Fatal(err)
	}
	if !*cmdInit {
		cfg.validate()
	}

	// Read secrets

	if _, err := os.Stat(*secretsPath); os.IsNotExist(err) {
		generateSecrets(*secretsPath)
	}
	secrets, err := ReadSecrets(*secretsPath)
	if err != nil {
		log.Fatal(err)
	}

	// Open database

	db, err := database.Open(*dbPath)
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	// Setup app context

	ctx := context.Background()
	ctx, cancel := signal.NotifyContext(ctx, os.Interrupt)
	defer cancel()

	group, ctx := errgroup.WithContext(ctx)

	// Setup gRPC client

	grpcAddr := cfg.Ghidra.GRPCAddr
	if grpcAddr == "" {
		grpcAddr = ghidra.DefaultGrpcAddr
	}
	client, err := ghidra.Connect(grpcAddr)
	if err != nil {
		log.Fatal(err)
	}

	// Setup web server

	if *cmdInit {
		return
	}

	redirectURL := cfg.BaseURL + "/redirect"

	// Legacy Discord setup (for backwards compatibility)
	var app *discord.Application
	var auth *discord.Auth

	// Check if using legacy Discord config
	if cfg.Discord.ClientID != "" && cfg.Discord.ClientSecret != "" {
		app, err = discord.GetApplication(ctx, cfg.Discord.BotToken)
		if err != nil {
			log.Fatal(err)
		}
		auth = discord.NewAuth(cfg.Discord.ClientID, cfg.Discord.ClientSecret, redirectURL)
	}

	// Calculate token validity
	tokenValidity := time.Duration(cfg.TokenValidityDays) * 24 * time.Hour
	if tokenValidity <= 0 {
		tokenValidity = 90 * 24 * time.Hour
	}

	issuer := token.NewIssuer(secrets.HMACSecret, tokenValidity)

	// Auto-discover GeoIP Database if not explicitly set
	geoIPPath := cfg.GeoIPDatabase
	if geoIPPath == "" {
		// Common locations to check
		candidatePaths := []string{
			"/data/GeoLite2-City.mmdb",
			"GeoLite2-City.mmdb",
		}
		for _, path := range candidatePaths {
			if _, err := os.Stat(path); err == nil {
				geoIPPath = path
				break
			}
		}

		// If still empty but credentials are set, default to standard location
		if geoIPPath == "" && cfg.MaxMindAccountID != "" {
			geoIPPath = "/data/GeoLite2-City.mmdb"
		}
	}

	webConfig := web.Config{
		CommunityName:     cfg.CommunityName,
		BaseURL:           cfg.BaseURL,
		GhidraEndpoint:    &cfg.Ghidra.Endpoint,
		Links:             cfg.Links,
		DiscordApp:        app,
		DiscordWebhookURL: cfg.Discord.WebhookURL,
		Dev:               *dev,
		SuperAdmins:       cfg.SuperAdmins,
		FirstUserIsAdmin:  cfg.FirstUserIsAdmin,
		GeoIPDatabase:     geoIPPath,
	}
	server, err := web.NewServer(&webConfig, db, auth, &issuer, client)
	if err != nil {
		log.Fatal(err)
	}

	// Configure OAuth providers dynamically
	for name, providerCfg := range cfg.OAuth {
		if !providerCfg.Enabled {
			continue
		}

		// Determine provider type
		providerType := providerCfg.Type
		if providerType == "" {
			if providerCfg.IssuerURL != "" {
				providerType = "oidc"
			} else if providerCfg.AuthURL != "" && providerCfg.TokenURL != "" {
				providerType = "oauth2"
			} else {
				log.Fatalf("oauth.%s: must specify type or provide issuer_url/auth_url", name)
			}
		}

		// Create provider based on type
		var provider oauth.Provider
		var providerErr error

		switch providerType {
		case "oauth2":
			provider = oauth.NewGenericOAuth2Provider(
				providerCfg.ClientID,
				providerCfg.ClientSecret,
				redirectURL,
				oauth.OAuth2Config{
					Name:           name,
					AuthURL:        providerCfg.AuthURL,
					TokenURL:       providerCfg.TokenURL,
					UserInfoURL:    providerCfg.UserInfoURL,
					Scopes:         providerCfg.Scopes,
					AuthStyle:      providerCfg.AuthStyle,
					UserIDField:    providerCfg.UserIDField,
					UsernameField:  providerCfg.UsernameField,
					AvatarField:    providerCfg.AvatarField,
					UserIDIsString: providerCfg.UserIDIsString,
				},
			)
		case "oidc":
			provider, providerErr = oauth.NewOIDCProvider(
				ctx,
				name,
				providerCfg.IssuerURL,
				providerCfg.ClientID,
				providerCfg.ClientSecret,
				redirectURL,
			)
			if providerErr != nil {
				log.Fatalf("Failed to create OIDC provider '%s': %v", name, providerErr)
			}
		default:
			log.Fatalf("oauth.%s: unknown provider type '%s' (must be 'oauth2' or 'oidc')", name, providerType)
		}

		// Create provider metadata
		displayName := providerCfg.DisplayName
		if displayName == "" {
			// Capitalize first letter of name as default
			displayName = name
			if len(displayName) > 0 {
				displayName = string(displayName[0]-32) + displayName[1:]
			}
		}

		metadata := &common.ProviderMetadata{
			Name:        name,
			DisplayName: displayName,
			IconURL:     providerCfg.IconURL,
		}

		server.AddProviderWithMetadata(provider, metadata)
		log.Printf("Enabled OAuth provider: %s (type: %s)", name, providerType)
	}

	// Load GeoIP database if configured (optional)
	if webConfig.GeoIPDatabase != "" {
		if err := server.LoadGeoIPDatabase(webConfig.GeoIPDatabase); err != nil {
			log.Printf("Warning: Failed to load GeoIP database: %v (location lookups disabled)", err)
		} else {
			log.Printf("GeoIP database loaded: %s", webConfig.GeoIPDatabase)
		}
	}

	mux := http.NewServeMux()
	handler := server.RegisterRoutes(mux)

	// Start periodic audit log cleanup (runs daily)
	retentionDays := cfg.AuditLogRetentionDays
	if retentionDays <= 0 {
		retentionDays = 90 // default
	}
	log.Printf("Audit log retention: %d days", retentionDays)

	go func() {
		ticker := time.NewTicker(24 * time.Hour)
		defer ticker.Stop()

		// Run cleanup immediately on startup
		cleanupAuditLogs(db, retentionDays)

		for range ticker.C {
			cleanupAuditLogs(db, retentionDays)
		}
	}()

	log.Println("Server listening on", *listen)

	httpServer := http.Server{
		Addr:    *listen,
		Handler: handler,
	}
	go func() {
		err := httpServer.ListenAndServe()
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatal(err)
		}
	}()
	group.Go(func() error {
		<-ctx.Done()
		return httpServer.Shutdown(ctx)
	})

	if err := group.Wait(); err != nil {
		log.Println(err)
	}
	log.Println("Server stopped gracefully")
}

func updateAccount(dbPath string, userID uint64, user, pass string) {
	db, err := database.Open(dbPath)
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	ctx := context.Background()
	// Default to "discord" provider for backwards compatibility
	provider := "discord"
	if err := db.UpdateAccount(ctx, userID, user, pass, provider); err != nil {
		log.Fatal(err)
	}
}

func cleanupAuditLogs(db *database.DB, retentionDays int) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Use default of 90 days if not configured
	if retentionDays <= 0 {
		retentionDays = 90
	}

	deleted, err := db.CleanupOldAuditLogs(ctx, retentionDays)
	if err != nil {
		log.Printf("Warning: Failed to cleanup old audit logs: %v", err)
	} else if deleted > 0 {
		log.Printf("Cleaned up %d old audit log entries (>%d days)", deleted, retentionDays)
	}
}
