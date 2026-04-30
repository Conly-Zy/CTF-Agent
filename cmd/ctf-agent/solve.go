package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"time"

	"github.com/Conly-Zy/CTF-Agent/internal/agent"
	"github.com/Conly-Zy/CTF-Agent/internal/config"
	"github.com/Conly-Zy/CTF-Agent/internal/llm"
	"github.com/Conly-Zy/CTF-Agent/internal/tools"
	"github.com/Conly-Zy/CTF-Agent/internal/tools/common"
	"github.com/spf13/cobra"
)

var solveCmd = &cobra.Command{
	Use:   "solve [flags]",
	Short: "Solve a CTF challenge",
	Long:  "Analyze and solve a CTF challenge automatically",
	RunE:  runSolve,
}

var (
	challengeType string
	description   string
	target        string
	files         []string
	configPath    string
)

func init() {
	solveCmd.Flags().StringVarP(&challengeType, "type", "t", "", "Challenge type (web, pwn, crypto, reverse)")
	solveCmd.Flags().StringVarP(&description, "description", "d", "", "Challenge description")
	solveCmd.Flags().StringVar(&target, "target", "", "Target URL or host")
	solveCmd.Flags().StringSliceVarP(&files, "files", "f", []string{}, "Challenge files")
	solveCmd.Flags().StringVarP(&configPath, "config", "c", "", "Config file path")

	rootCmd.AddCommand(solveCmd)
}

func runSolve(cmd *cobra.Command, args []string) error {
	cfg, err := config.Load(configPath)
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	if err := cfg.Validate(); err != nil {
		return fmt.Errorf("invalid config: %w", err)
	}

	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))

	llmClient, err := llm.NewClient(cfg.Anthropic.APIKey, cfg.Anthropic.Model)
	if err != nil {
		return fmt.Errorf("create LLM client: %w", err)
	}

	registry := tools.NewRegistry()

	fileReadTool := common.NewFileReadTool()
	if err := registry.Register(fileReadTool); err != nil {
		return fmt.Errorf("register file_read tool: %w", err)
	}

	fileWriteTool := common.NewFileWriteTool()
	if err := registry.Register(fileWriteTool); err != nil {
		return fmt.Errorf("register file_write tool: %w", err)
	}

	shellExecTool := common.NewShellExecTool(30 * time.Second)
	if err := registry.Register(shellExecTool); err != nil {
		return fmt.Errorf("register shell_exec tool: %w", err)
	}

	orchestrator := agent.NewOrchestrator(llmClient, registry, logger, cfg.Agent.MaxIterations, cfg.Agent.Timeout)

	req := agent.SolveRequest{
		ChallengeType: challengeType,
		Description:   description,
		Target:        target,
		Files:         files,
	}

	fmt.Printf("Starting CTF-Agent solver...\n")
	fmt.Printf("Challenge Type: %s\n", req.ChallengeType)
	fmt.Printf("Target: %s\n\n", req.Target)

	ctx := context.Background()
	result, err := orchestrator.Solve(ctx, req)
	if err != nil {
		return fmt.Errorf("solve failed: %w", err)
	}

	fmt.Printf("\n=== Result ===\n")
	fmt.Printf("Success: %v\n", result.Success)
	if result.Flag != "" {
		fmt.Printf("Flag: %s\n", result.Flag)
	}
	fmt.Printf("Iterations: %d\n", result.Iterations)
	fmt.Printf("Duration: %v\n", result.Duration)

	if !result.Success {
		return fmt.Errorf("failed to find flag: %w", result.Error)
	}

	return nil
}
