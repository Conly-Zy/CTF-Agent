package crypto

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
)

type HashIDTool struct{}

func NewHashIDTool() *HashIDTool {
	return &HashIDTool{}
}

func (t *HashIDTool) Name() string {
	return "hash_id"
}

func (t *HashIDTool) Description() string {
	return "识别哈希类型。根据哈希值的长度和格式推断可能的算法。"
}

func (t *HashIDTool) Schema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"hash": map[string]any{
				"type":        "string",
				"description": "要识别的哈希值",
			},
		},
		"required": []string{"hash"},
	}
}

type hashInfo struct {
	Name     string
	Length   int
	Pattern  string
}

var hashTypes = []hashInfo{
	{"MD5", 32, `^[a-f0-9]{32}$`},
	{"MD4", 32, `^[a-f0-9]{32}$`},
	{"MD2", 32, `^[a-f0-9]{32}$`},
	{"SHA-1", 40, `^[a-f0-9]{40}$`},
	{"SHA-224", 56, `^[a-f0-9]{56}$`},
	{"SHA-256", 64, `^[a-f0-9]{64}$`},
	{"SHA-384", 96, `^[a-f0-9]{96}$`},
	{"SHA-512", 128, `^[a-f0-9]{128}$`},
	{"SHA3-256", 64, `^[a-f0-9]{64}$`},
	{"SHA3-512", 128, `^[a-f0-9]{128}$`},
	{"RIPEMD-160", 40, `^[a-f0-9]{40}$`},
	{"CRC32", 8, `^[a-f0-9]{8}$`},
	{"Adler-32", 8, `^[a-f0-9]{8}$`},
	{"NTLM", 32, `^[a-f0-9]{32}$`},
	{"LM", 32, `^[a-f0-9]{32}$`},
	{"MySQL323", 16, `^[a-f0-9]{16}$`},
	{"MySQL4.1+", 40, `^[a-f0-9]{40}$`},
	{"Blowfish", 60, `^\$2[aby]?\$[0-9]{2}\$[./A-Za-z0-9]{53}$`},
	{"bcrypt", 60, `^\$2[aby]?\$[0-9]{2}\$[./A-Za-z0-9]{53}$`},
	{"DES", 13, `^[./A-Za-z0-9]{13}$`},
	{"MD5 (Unix)", 34, `^\$1\$[./A-Za-z0-9]{1,8}\$[./A-Za-z0-9]{22}$`},
	{"SHA-512 (Unix)", 98, `^\$6\$[./A-Za-z0-9]{1,16}\$[./A-Za-z0-9]{86}$`},
}

func (t *HashIDTool) Execute(ctx context.Context, input json.RawMessage) (string, error) {
	var params struct {
		Hash string `json:"hash"`
	}
	if err := json.Unmarshal(input, &params); err != nil {
		return "", fmt.Errorf("parse input: %w", err)
	}

	hash := params.Hash
	var matches []string

	for _, ht := range hashTypes {
		if len(hash) == ht.Length || ht.Pattern != "" {
			if ht.Pattern != "" {
				if matched, _ := regexp.MatchString(ht.Pattern, hash); matched {
					matches = append(matches, ht.Name)
				}
			} else if len(hash) == ht.Length {
				matches = append(matches, ht.Name)
			}
		}
	}

	if len(matches) == 0 {
		return fmt.Sprintf("无法识别哈希类型 (长度: %d)", len(hash)), nil
	}

	result := fmt.Sprintf("哈希长度: %d\n可能的类型:\n", len(hash))
	for _, m := range matches {
		result += fmt.Sprintf("  - %s\n", m)
	}
	return result, nil
}
