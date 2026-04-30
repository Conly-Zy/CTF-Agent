package main

import (
	"fmt"
	"log/slog"
	"os"
	"time"

	"github.com/Conly-Zy/CTF-Agent/internal/agent"
	"github.com/Conly-Zy/CTF-Agent/internal/api"
	"github.com/Conly-Zy/CTF-Agent/internal/config"
	"github.com/Conly-Zy/CTF-Agent/internal/llm"
	"github.com/Conly-Zy/CTF-Agent/internal/store"
	"github.com/Conly-Zy/CTF-Agent/internal/tools"
	"github.com/Conly-Zy/CTF-Agent/internal/tools/common"
	"github.com/spf13/cobra"
)

var serverCmd = &cobra.Command{
	Use:   "server",
	Short: "Start the web server",
	Long:  "Start the HTTP API server with web interface",
	RunE:  runServer,
}

var (
	serverAddr   string
	serverConfig string
	dbPath       string
)

func init() {
	serverCmd.Flags().StringVar(&serverAddr, "addr", ":8080", "Server address")
	serverCmd.Flags().StringVarP(&serverConfig, "config", "c", "", "Config file path")
	serverCmd.Flags().StringVar(&dbPath, "db", "ctf-agent.db", "Database path")

	rootCmd.AddCommand(serverCmd)
}

func runServer(cmd *cobra.Command, args []string) error {
	cfg, err := config.Load(serverConfig)
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))

	db, err := store.NewSQLiteStore(dbPath)
	if err != nil {
		return fmt.Errorf("open database: %w", err)
	}
	defer db.Close()

	// Initialize tools
	registry := tools.NewRegistry()
	registry.Register(common.NewFileReadTool())
	registry.Register(common.NewFileWriteTool())
	registry.Register(common.NewShellExecTool(30 * time.Second))

	// Initialize LLM client
	llmClient, err := llm.NewClient(cfg.Anthropic.APIKey, cfg.Anthropic.Model)
	if err != nil {
		logger.Warn("LLM client not configured", "error", err)
	}

	// Initialize orchestrator
	var orch *agent.Orchestrator
	if llmClient != nil {
		orch = agent.NewOrchestrator(llmClient, registry, logger, cfg.Agent.MaxIterations, cfg.Agent.Timeout)
	}

	// Create server
	server := api.NewServer(db, logger, serverAddr)
	if orch != nil {
		server.SetOrchestrator(orch)
	}
	server.SetRegistry(registry)

	fmt.Printf("Starting CTF-Agent web server...\n")
	fmt.Printf("Address: %s\n", serverAddr)
	fmt.Printf("Database: %s\n", dbPath)
	if cfg.Anthropic.APIKey == "" {
		fmt.Printf("WARNING: ANTHROPIC_API_KEY not set - solving will not work\n")
	}

	return server.Start()
}
