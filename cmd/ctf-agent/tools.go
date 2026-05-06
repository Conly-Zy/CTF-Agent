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
	r.RegisterWithGroup(common.NewFileReadTool(), tools.GroupCommon)
	r.RegisterWithGroup(common.NewFileWriteTool(), tools.GroupCommon)
	r.RegisterWithGroup(common.NewShellExecTool(30*time.Second), tools.GroupCommon)
}

func registerWebTools(r *tools.Registry) {
	r.RegisterWithGroup(web.NewHTTPRequestTool(30*time.Second), tools.GroupWeb)
	r.RegisterWithGroup(web.NewDirScanTool(60*time.Second), tools.GroupWeb)
}

func registerPwnTools(r *tools.Registry) {
	r.RegisterWithGroup(pwn.NewBinaryInfoTool(10*time.Second), tools.GroupPwn)
	r.RegisterWithGroup(pwn.NewDisassembleTool(15*time.Second), tools.GroupPwn)
	r.RegisterWithGroup(pwn.NewPatternTool(), tools.GroupPwn)
}

func registerCryptoTools(r *tools.Registry) {
	r.RegisterWithGroup(crypto.NewEncodeDecodeTool(), tools.GroupCrypto)
	r.RegisterWithGroup(crypto.NewHashIDTool(), tools.GroupCrypto)
	r.RegisterWithGroup(crypto.NewMathTool(), tools.GroupCrypto)
}

func registerReverseTools(r *tools.Registry) {
	r.RegisterWithGroup(reverse.NewStringsTool(15*time.Second), tools.GroupReverse)
	r.RegisterWithGroup(reverse.NewHexDumpTool(), tools.GroupReverse)
	r.RegisterWithGroup(reverse.NewEntropyTool(), tools.GroupReverse)
}
