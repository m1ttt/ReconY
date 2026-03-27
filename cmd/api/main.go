package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	"reconx/internal/api"
	"reconx/internal/api/ws"
	"reconx/internal/config"
	"reconx/internal/db"
	"reconx/internal/engine"
	phase1 "reconx/internal/scanner/phase1_passive"
	phase2 "reconx/internal/scanner/phase2_subdomains"
	phase3 "reconx/internal/scanner/phase3_ports"
	phase4 "reconx/internal/scanner/phase4_fingerprint"
	phase5 "reconx/internal/scanner/phase5_content"
	phase6 "reconx/internal/scanner/phase6_cloud"
	phase7 "reconx/internal/scanner/phase7_vulns"
	"reconx/internal/wordlist"
	"reconx/internal/workflow"
)

func intPtrString(v *int) string {
	if v == nil {
		return "default"
	}
	return fmt.Sprintf("%d", *v)
}

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("loading config: %w", err)
	}

	database, err := db.Open(cfg.General.DBPath)
	if err != nil {
		return fmt.Errorf("opening database: %w", err)
	}
	defer database.Close()

	if err := database.Migrate(); err != nil {
		return fmt.Errorf("running migrations: %w", err)
	}

	eventBus := engine.NewEventBus()
	eng := engine.New(database.DB, cfg, eventBus)
	registerTools(eng, cfg)

	home, _ := os.UserHomeDir()
	customDir := filepath.Join(home, ".reconx", "workflows")
	wfRegistry, err := workflow.NewRegistry(customDir)
	if err != nil {
		return fmt.Errorf("loading workflows: %w", err)
	}

	// Create API server
	srv := &api.Server{
		DB:        database.DB,
		Engine:    eng,
		Config:    cfg,
		EventBus:  eventBus,
		Workflows: wfRegistry,
	}

	router := srv.NewRouter()

	// Mount WebSocket hub
	hub := ws.NewHub(eventBus)
	router.Get("/api/v1/ws", hub.ServeHTTP)

	// Serve compiled React SPA for all non-API routes
	mountSPA(router, "")

	// Start HTTP server
	addr := cfg.General.APIListenAddr
	ffufCfg := cfg.GetToolConfig("ffuf")
	feroxCfg := cfg.GetToolConfig("feroxbuster")
	httpxCfg := cfg.GetToolConfig("httpx")
	log.Printf("reconx-api starting on %s", addr)
	log.Printf("Config loaded: db=%s ffuf(timeout=%s,threads=%s) ferox(timeout=%s,threads=%s) httpx(timeout=%s,threads=%s)",
		cfg.General.DBPath,
		ffufCfg.Timeout, intPtrString(ffufCfg.Threads),
		feroxCfg.Timeout, intPtrString(feroxCfg.Threads),
		httpxCfg.Timeout, intPtrString(httpxCfg.Threads),
	)
	log.Printf("WebSocket endpoint: ws://%s/api/v1/ws", addr)

	httpServer := &http.Server{
		Addr:    addr,
		Handler: router,
	}

	// Graceful shutdown
	go func() {
		sigCh := make(chan os.Signal, 1)
		signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
		<-sigCh
		log.Println("Shutting down...")
		httpServer.Close()
	}()

	if err := httpServer.ListenAndServe(); err != http.ErrServerClosed {
		return err
	}

	return nil
}

func registerTools(eng *engine.Engine, cfg *config.Config) {
	eng.RegisterTool(&phase1.WhoisRunner{})
	eng.RegisterTool(&phase1.DNSRunner{})
	eng.RegisterTool(&phase1.WaybackURLsRunner{})
	eng.RegisterTool(&phase1.GAURunner{})

	eng.RegisterTool(&phase2.SubfinderRunner{})
	eng.RegisterTool(&phase2.CrtshRunner{})
	eng.RegisterTool(&phase2.AmassRunner{})
	eng.RegisterTool(&phase2.PureDNSRunner{})

	eng.RegisterTool(&phase3.NmapRunner{})
	eng.RegisterTool(&phase3.ShodanRunner{})
	eng.RegisterTool(&phase3.CensysRunner{})

	eng.RegisterTool(&phase4.HTTPXRunner{})
	eng.RegisterTool(&phase4.WAFDetectRunner{})
	eng.RegisterTool(&phase4.SSLAnalyzeRunner{})
	eng.RegisterTool(&phase4.ClassifyRunner{})

	resolver := wordlist.NewResolver(cfg.General.SecListsPath)
	selector := wordlist.NewSelector(resolver, cfg.Wordlists)
	eng.RegisterTool(&phase5.KatanaRunner{})
	eng.RegisterTool(&phase5.JSluiceRunner{})
	eng.RegisterTool(&phase5.SecretFinderRunner{})
	eng.RegisterTool(&phase5.ParamSpiderRunner{})
	eng.RegisterTool(&phase5.FFUFRunner{Selector: selector, Resolver: resolver})
	eng.RegisterTool(&phase5.FeroxbusterRunner{Selector: selector, Resolver: resolver})
	eng.RegisterTool(&phase5.CMSeekRunner{})
	eng.RegisterTool(&phase5.GoWitnessRunner{})
	eng.RegisterTool(&phase5.StaticAnalysisRunner{})
	eng.RegisterTool(&phase5.AIResearchRunner{})

	eng.RegisterTool(&phase6.BucketEnumRunner{})
	eng.RegisterTool(&phase6.GitDorkRunner{})
	eng.RegisterTool(&phase6.JSSecretsRunner{})

	eng.RegisterTool(&phase7.NucleiRunner{})
}
