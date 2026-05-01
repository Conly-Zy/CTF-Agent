package reverse

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
)

type HexDumpTool struct{}

func NewHexDumpTool() *HexDumpTool {
	return &HexDumpTool{}
}

func (t *HexDumpTool) Name() string {
	return "hex_dump"
}

func (t *HexDumpTool) Description() string {
	return "以十六进制形式查看文件内容，同时显示 ASCII 字符。支持指定偏移量和长度。"
}

func (t *HexDumpTool) Schema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"path": map[string]any{
				"type":        "string",
				"description": "文件路径",
			},
			"offset": map[string]any{
				"type":        "integer",
				"description": "起始偏移量 (字节，默认 0)",
				"default":     0,
			},
			"length": map[string]any{
				"type":        "integer",
				"description": "读取长度 (字节，默认 512)",
				"default":     512,
			},
		},
		"required": []string{"path"},
	}
}

func (t *HexDumpTool) Execute(ctx context.Context, input json.RawMessage) (string, error) {
	var params struct {
		Path   string `json:"path"`
		Offset int64  `json:"offset"`
		Length int    `json:"length"`
	}
	if err := json.Unmarshal(input, &params); err != nil {
		return "", fmt.Errorf("parse input: %w", err)
	}

	if params.Length <= 0 {
		params.Length = 512
	}
	if params.Length > 4096 {
		params.Length = 4096
	}

	f, err := os.Open(params.Path)
	if err != nil {
		return "", fmt.Errorf("open file: %w", err)
	}
	defer f.Close()

	if params.Offset > 0 {
		f.Seek(params.Offset, 0)
	}

	buf := make([]byte, params.Length)
	n, err := f.Read(buf)
	if err != nil && n == 0 {
		return "", fmt.Errorf("read file: %w", err)
	}
	buf = buf[:n]

	return formatHexDump(buf, params.Offset), nil
}

func formatHexDump(data []byte, baseOffset int64) string {
	var result string
	for i := 0; i < len(data); i += 16 {
		end := i + 16
		if end > len(data) {
			end = len(data)
		}

		line := data[i:end]

		// Offset
		result += fmt.Sprintf("%08x  ", baseOffset+int64(i))

		// Hex bytes
		for j := 0; j < 16; j++ {
			if j < len(line) {
				result += fmt.Sprintf("%02x ", line[j])
			} else {
				result += "   "
			}
			if j == 7 {
				result += " "
			}
		}

		result += " |"

		// ASCII
		for _, b := range line {
			if b >= 32 && b <= 126 {
				result += string(rune(b))
			} else {
				result += "."
			}
		}

		result += "|\n"
	}

	return result
}
