package crypto

import (
	"context"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
)

type EncodeDecodeTool struct{}

func NewEncodeDecodeTool() *EncodeDecodeTool {
	return &EncodeDecodeTool{}
}

func (t *EncodeDecodeTool) Name() string {
	return "encode_decode"
}

func (t *EncodeDecodeTool) Description() string {
	return "编解码工具：支持 Base64、Hex、ROT13、URL 编解码，以及 XOR 运算。"
}

func (t *EncodeDecodeTool) Schema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"action": map[string]any{
				"type":        "string",
				"description": "操作: encode 或 decode",
				"enum":        []string{"encode", "decode"},
			},
			"format": map[string]any{
				"type":        "string",
				"description": "编码格式: base64, hex, rot13, url, xor",
				"enum":        []string{"base64", "hex", "rot13", "url", "xor"},
			},
			"data": map[string]any{
				"type":        "string",
				"description": "要处理的数据",
			},
			"key": map[string]any{
				"type":        "string",
				"description": "XOR 密钥 (仅 xor 格式需要)",
			},
		},
		"required": []string{"action", "format", "data"},
	}
}

func (t *EncodeDecodeTool) Execute(ctx context.Context, input json.RawMessage) (string, error) {
	var params struct {
		Action string `json:"action"`
		Format string `json:"format"`
		Data   string `json:"data"`
		Key    string `json:"key"`
	}
	if err := json.Unmarshal(input, &params); err != nil {
		return "", fmt.Errorf("parse input: %w", err)
	}

	switch params.Format {
	case "base64":
		return t.base64(params.Action, params.Data)
	case "hex":
		return t.hex(params.Action, params.Data)
	case "rot13":
		return rot13(params.Data), nil
	case "url":
		return t.url(params.Action, params.Data)
	case "xor":
		return t.xor(params.Data, params.Key)
	default:
		return "", fmt.Errorf("unsupported format: %s", params.Format)
	}
}

func (t *EncodeDecodeTool) base64(action, data string) (string, error) {
	if action == "encode" {
		return base64.StdEncoding.EncodeToString([]byte(data)), nil
	}
	result, err := base64.StdEncoding.DecodeString(data)
	if err != nil {
		return "", fmt.Errorf("base64 decode failed: %w", err)
	}
	return string(result), nil
}

func (t *EncodeDecodeTool) hex(action, data string) (string, error) {
	if action == "encode" {
		return hex.EncodeToString([]byte(data)), nil
	}
	result, err := hex.DecodeString(strings.TrimPrefix(data, "0x"))
	if err != nil {
		return "", fmt.Errorf("hex decode failed: %w", err)
	}
	return string(result), nil
}

func (t *EncodeDecodeTool) url(action, data string) (string, error) {
	var result strings.Builder
	if action == "encode" {
		for _, b := range []byte(data) {
			if (b >= 'A' && b <= 'Z') || (b >= 'a' && b <= 'z') || (b >= '0' && b <= '9') || b == '-' || b == '_' || b == '.' || b == '~' {
				result.WriteByte(b)
			} else {
				result.WriteString(fmt.Sprintf("%%%02X", b))
			}
		}
		return result.String(), nil
	}
	// Decode
	var decoded []byte
	for i := 0; i < len(data); i++ {
		if data[i] == '%' && i+2 < len(data) {
			var b byte
			fmt.Sscanf(data[i+1:i+3], "%02X", &b)
			decoded = append(decoded, b)
			i += 2
		} else if data[i] == '+' {
			decoded = append(decoded, ' ')
		} else {
			decoded = append(decoded, data[i])
		}
	}
	return string(decoded), nil
}

func (t *EncodeDecodeTool) xor(data, key string) (string, error) {
	if key == "" {
		return "", fmt.Errorf("XOR key is required")
	}
	keyBytes := []byte(key)
	dataBytes := []byte(data)
	result := make([]byte, len(dataBytes))
	for i, b := range dataBytes {
		result[i] = b ^ keyBytes[i%len(keyBytes)]
	}
	// Return both hex and printable representation
	hexStr := hex.EncodeToString(result)
	isPrintable := true
	for _, b := range result {
		if b < 32 || b > 126 {
			isPrintable = false
			break
		}
	}
	if isPrintable {
		return fmt.Sprintf("Hex: %s\nText: %s", hexStr, string(result)), nil
	}
	return fmt.Sprintf("Hex: %s\n(non-printable)", hexStr), nil
}

func rot13(s string) string {
	var result strings.Builder
	for _, c := range s {
		if c >= 'a' && c <= 'z' {
			result.WriteRune('a' + (c-'a'+13)%26)
		} else if c >= 'A' && c <= 'Z' {
			result.WriteRune('A' + (c-'A'+13)%26)
		} else {
			result.WriteRune(c)
		}
	}
	return result.String()
}
