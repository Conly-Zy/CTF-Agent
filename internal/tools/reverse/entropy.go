package reverse

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"os"
)

type EntropyTool struct{}

func NewEntropyTool() *EntropyTool {
	return &EntropyTool{}
}

func (t *EntropyTool) Name() string {
	return "entropy"
}

func (t *EntropyTool) Description() string {
	return "计算文件或数据块的熵值。高熵值 (>7.0) 通常表示加密或压缩数据。用于识别加密段、嵌入的密钥等。"
}

func (t *EntropyTool) Schema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"path": map[string]any{
				"type":        "string",
				"description": "文件路径",
			},
			"window": map[string]any{
				"type":        "integer",
				"description": "滑动窗口大小 (字节，默认 0 表示全文件)",
				"default":     0,
			},
		},
		"required": []string{"path"},
	}
}

func (t *EntropyTool) Execute(ctx context.Context, input json.RawMessage) (string, error) {
	var params struct {
		Path   string `json:"path"`
		Window int    `json:"window"`
	}
	if err := json.Unmarshal(input, &params); err != nil {
		return "", fmt.Errorf("parse input: %w", err)
	}

	data, err := os.ReadFile(params.Path)
	if err != nil {
		return "", fmt.Errorf("read file: %w", err)
	}

	if params.Window <= 0 {
		// Whole file
		e := calculateEntropy(data)
		return fmt.Sprintf("文件: %s\n大小: %d bytes\n熵值: %.4f / 8.0\n\n%s",
			params.Path, len(data), e, entropyInterpretation(e)), nil
	}

	// Sliding window analysis
	var result string
	windowSize := params.Window
	if windowSize > len(data) {
		windowSize = len(data)
	}

	for i := 0; i <= len(data)-windowSize; i += windowSize {
		e := calculateEntropy(data[i : i+windowSize])
		result += fmt.Sprintf("0x%08x: %.4f\n", i, e)
	}

	return result, nil
}

func calculateEntropy(data []byte) float64 {
	if len(data) == 0 {
		return 0
	}

	var freq [256]float64
	for _, b := range data {
		freq[b]++
	}

	size := float64(len(data))
	entropy := 0.0
	for _, f := range freq {
		if f > 0 {
			p := f / size
			entropy -= p * math.Log2(p)
		}
	}

	return entropy
}

func entropyInterpretation(e float64) string {
	switch {
	case e < 1.0:
		return "极低熵：数据几乎全是重复字节"
	case e < 3.5:
		return "低熵：可能是文本或结构化数据"
	case e < 5.0:
		return "中熵：可能是代码或混合内容"
	case e < 6.5:
		return "中高熵：可能包含压缩数据或某些加密"
	case e < 7.0:
		return "高熵：很可能是压缩或加密数据"
	default:
		return "极高熵：强加密数据或随机数据"
	}
}
