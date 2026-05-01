package main

import (
	"time"

	"github.com/Conly-Zy/CTF-Agent/internal/agent"
	"github.com/Conly-Zy/CTF-Agent/internal/store"
	"github.com/Conly-Zy/CTF-Agent/internal/tools"
	"github.com/Conly-Zy/CTF-Agent/internal/tools/common"
	"github.com/Conly-Zy/CTF-Agent/internal/tools/crypto"
	"github.com/Conly-Zy/CTF-Agent/internal/tools/pwn"
	"github.com/Conly-Zy/CTF-Agent/internal/tools/reverse"
	"github.com/Conly-Zy/CTF-Agent/internal/tools/web"
)

// knowledgeAdapter adapts store.SQLiteStore to agent.KnowledgeStore interface.
type knowledgeAdapter struct {
	store *store.SQLiteStore
}

func (a *knowledgeAdapter) SearchKnowledgeByType(challengeType string, limit int) ([]agent.KnowledgeEntry, error) {
	items, err := a.store.SearchKnowledgeByType(challengeType, limit)
	if err != nil {
		return nil, err
	}
	entries := make([]agent.KnowledgeEntry, len(items))
	for i, k := range items {
		entries[i] = agent.KnowledgeEntry{
			Title:   k.Title,
			Content: k.Content,
			Type:    k.Type,
		}
	}
	return entries, nil
}

func registerCommonTools(r *tools.Registry) {
	r.Register(common.NewFileReadTool())
	r.Register(common.NewFileWriteTool())
	r.Register(common.NewShellExecTool(30 * time.Second))
}

func registerWebTools(r *tools.Registry) {
	r.Register(web.NewHTTPRequestTool(30 * time.Second))
	r.Register(web.NewDirScanTool(60 * time.Second))
}

func registerPwnTools(r *tools.Registry) {
	r.Register(pwn.NewBinaryInfoTool(10 * time.Second))
	r.Register(pwn.NewDisassembleTool(15 * time.Second))
	r.Register(pwn.NewPatternTool())
}

func registerCryptoTools(r *tools.Registry) {
	r.Register(crypto.NewEncodeDecodeTool())
	r.Register(crypto.NewHashIDTool())
	r.Register(crypto.NewMathTool())
}

func registerReverseTools(r *tools.Registry) {
	r.Register(reverse.NewStringsTool(15 * time.Second))
	r.Register(reverse.NewHexDumpTool())
	r.Register(reverse.NewEntropyTool())
}
