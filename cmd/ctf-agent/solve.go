package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"

	"github.com/Conly-Zy/CTF-Agent/internal/agent"
	"github.com/Conly-Zy/CTF-Agent/internal/config"
	"github.com/Conly-Zy/CTF-Agent/internal/llm"
	"github.com/Conly-Zy/CTF-Agent/internal/tools"
	"github.com/spf13/cobra"
)

var solveCmd = &cobra.Command{
	Use:   "solve [flags]",
	Short: "解题",
	Long:  "自动分析和解决 CTF 题目",
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
	solveCmd.Flags().StringVarP(&challengeType, "type", "t", "", "题目类型 (web, pwn, crypto, reverse)")
	solveCmd.Flags().StringVarP(&description, "description", "d", "", "题目描述")
	solveCmd.Flags().StringVar(&target, "target", "", "目标 URL 或主机")
	solveCmd.Flags().StringSliceVarP(&files, "files", "f", []string{}, "题目文件")
	solveCmd.Flags().StringVarP(&configPath, "config", "c", "", "配置文件路径")

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
	registerCommonTools(registry)
	registerWebTools(registry)
	registerPwnTools(registry)
	registerCryptoTools(registry)
	registerReverseTools(registry)

	orchestrator := agent.NewOrchestrator(llmClient, registry, logger, cfg.Agent.MaxIterations, cfg.Agent.Timeout)
	orchestrator.SetFlagPatterns(cfg.Flag.Patterns)

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
