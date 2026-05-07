package main

import (
	"embed"
	"fmt"
	"log/slog"
	"os"

	"github.com/Conly-Zy/CTF-Agent/internal/agent"
	"github.com/Conly-Zy/CTF-Agent/internal/agent/specialists"
	"github.com/Conly-Zy/CTF-Agent/internal/api"
	"github.com/Conly-Zy/CTF-Agent/internal/config"
	"github.com/Conly-Zy/CTF-Agent/internal/llm"
	"github.com/Conly-Zy/CTF-Agent/internal/store"
	"github.com/Conly-Zy/CTF-Agent/internal/tools"
	"github.com/spf13/cobra"
)

//go:embed all:web_dist
var frontendFS embed.FS

var serverCmd = &cobra.Command{
	Use:   "server",
	Short: "启动 Web 服务器",
	Long:  "启动 HTTP API 服务器和 Web 界面",
	RunE:  runServer,
}

var (
	serverAddr   string
	serverConfig string
	dbPath       string
)

func init() {
	serverCmd.Flags().StringVar(&serverAddr, "addr", envOrDefault("CTF_AGENT_ADDR", ":4399"), "服务器监听地址")
	serverCmd.Flags().StringVarP(&serverConfig, "config", "c", envOrDefault("CTF_AGENT_CONFIG", "config.yaml"), "配置文件路径")
	serverCmd.Flags().StringVar(&dbPath, "db", envOrDefault("CTF_AGENT_DB", "ctf-agent.db"), "数据库路径")

	rootCmd.AddCommand(serverCmd)
}

func envOrDefault(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
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
	registerCommonTools(registry)
	registerWebTools(registry)
	registerPwnTools(registry)
	registerCryptoTools(registry)
	registerReverseTools(registry)

	// Initialize LLM client with multi-provider support
	llmProvider, err := llm.NewProvider(llm.ProviderConfig{
		Type:    llm.ProviderType(cfg.GetProvider()),
		APIKey:  cfg.GetAPIKey(),
		Model:   cfg.GetModel(),
		BaseURL: cfg.LLM.BaseURL,
	})
	if err != nil {
		logger.Warn("LLM provider not configured", "error", err)
	}
	var llmClient *llm.Client
	if llmProvider != nil {
		llmClient = llm.NewClientWithProvider(llmProvider)
	}

	// Initialize orchestrator and primary agent
	var orch *agent.Orchestrator
	var primaryAgent *agent.PrimaryAgent
	if llmClient != nil {
		orch = agent.NewOrchestrator(llmClient, registry, logger, cfg.Agent.MaxIterations, cfg.Agent.Timeout)
		orch.SetFlagPatterns(cfg.Flag.Patterns)
		orch.SetKnowledgeStore(&knowledgeAdapter{store: db})

		// Create PrimaryAgent with specialist agents
		primaryAgent = agent.NewPrimaryAgent(llmClient, registry, logger, cfg.Agent.MaxIterations, cfg.Agent.Timeout)
		primaryAgent.RegisterAgent(specialists.NewWebAgent(llmClient, registry, logger, cfg.Agent.MaxIterations, cfg.Agent.Timeout))
		primaryAgent.RegisterAgent(specialists.NewPwnAgent(llmClient, registry, logger, cfg.Agent.MaxIterations, cfg.Agent.Timeout))
		primaryAgent.RegisterAgent(specialists.NewCryptoAgent(llmClient, registry, logger, cfg.Agent.MaxIterations, cfg.Agent.Timeout))
		primaryAgent.RegisterAgent(specialists.NewReverseAgent(llmClient, registry, logger, cfg.Agent.MaxIterations, cfg.Agent.Timeout))
	}

	// Create server
	server := api.NewServer(db, logger, serverAddr)
	if orch != nil {
		server.SetOrchestrator(orch)
	}
	if primaryAgent != nil {
		server.SetPrimaryAgent(primaryAgent)
	}
	if llmClient != nil {
		server.SetLLMClient(llmClient)
	}
	server.SetRegistry(registry)
	server.SetConfig(cfg)

	// Try to use embedded frontend, fallback to filesystem
	if _, err := frontendFS.Open("web_dist"); err == nil {
		server.SetFrontendFS(frontendFS)
		logger.Info("using embedded frontend")
	} else {
		logger.Info("using filesystem frontend (dev mode)")
	}

	fmt.Printf("Starting CTF-Agent web server...\n")
	fmt.Printf("Address: %s\n", serverAddr)
	fmt.Printf("Database: %s\n", dbPath)
	if cfg.GetAPIKey() == "" {
		fmt.Printf("WARNING: LLM API key not set - solving will not work\n")
	}

	return server.Start()
}
