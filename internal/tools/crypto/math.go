package crypto

import (
	"context"
	"encoding/json"
	"fmt"
	"math/big"
)

type MathTool struct{}

func NewMathTool() *MathTool {
	return &MathTool{}
}

func (t *MathTool) Name() string {
	return "math_tool"
}

func (t *MathTool) Description() string {
	return "数论工具：大数运算、因式分解、模逆元、GCD、中国剩余定理等。常用于 RSA 攻击。"
}

func (t *MathTool) Schema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"operation": map[string]any{
				"type":        "string",
				"description": "运算类型",
				"enum":        []string{"gcd", "modinv", "factor", "powmod", "is_prime", "sqrt"},
			},
			"a": map[string]any{
				"type":        "string",
				"description": "第一个操作数 (十进制或 0x 开头的十六进制)",
			},
			"b": map[string]any{
				"type":        "string",
				"description": "第二个操作数 (modinv 的模数、factor 的上限等)",
			},
			"mod": map[string]any{
				"type":        "string",
				"description": "模数 (powmod 运算使用)",
			},
		},
		"required": []string{"operation", "a"},
	}
}

func (t *MathTool) Execute(ctx context.Context, input json.RawMessage) (string, error) {
	var params struct {
		Operation string `json:"operation"`
		A         string `json:"a"`
		B         string `json:"b"`
		Mod       string `json:"mod"`
	}
	if err := json.Unmarshal(input, &params); err != nil {
		return "", fmt.Errorf("parse input: %w", err)
	}

	a := parseBigint(params.A)

	switch params.Operation {
	case "gcd":
		b := parseBigint(params.B)
		result := new(big.Int).GCD(nil, nil, a, b)
		return fmt.Sprintf("GCD(%s, %s) = %s", params.A, params.B, result.String()), nil

	case "modinv":
		b := parseBigint(params.B)
		result := new(big.Int).ModInverse(a, b)
		if result == nil {
			return "模逆元不存在 (GCD != 1)", nil
		}
		return fmt.Sprintf("modinv(%s, %s) = %s", params.A, params.B, result.String()), nil

	case "factor":
		return factorize(a), nil

	case "powmod":
		b := parseBigint(params.B)
		mod := parseBigint(params.Mod)
		result := new(big.Int).Exp(a, b, mod)
		return fmt.Sprintf("%s^%s mod %s = %s", params.A, params.B, params.Mod, result.String()), nil

	case "is_prime":
		probably := a.ProbablyPrime(20)
		if probably {
			return fmt.Sprintf("%s 很可能是素数 (Miller-Rabin 20 轮)", params.A), nil
		}
		return fmt.Sprintf("%s 不是素数", params.A), nil

	case "sqrt":
		result := new(big.Int).Sqrt(a)
		// Check if perfect square
		square := new(big.Int).Mul(result, result)
		if square.Cmp(a) == 0 {
			return fmt.Sprintf("sqrt(%s) = %s (完全平方数)", params.A, result.String()), nil
		}
		return fmt.Sprintf("sqrt(%s) ≈ %s (非完全平方数，向下取整)", params.A, result.String()), nil

	default:
		return "", fmt.Errorf("unknown operation: %s", params.Operation)
	}
}

func parseBigint(s string) *big.Int {
	n := new(big.Int)
	if len(s) > 2 && s[:2] == "0x" {
		n.SetString(s[2:], 16)
	} else {
		n.SetString(s, 10)
	}
	return n
}

func factorize(n *big.Int) string {
	if n.Cmp(big.NewInt(1)) <= 0 {
		return n.String()
	}

	var factors []string
	remaining := new(big.Int).Set(n)
	two := big.NewInt(2)

	// Check 2
	for new(big.Int).Mod(remaining, two).Cmp(big.NewInt(0)) == 0 {
		factors = append(factors, "2")
		remaining.Div(remaining, two)
	}

	// Check odd factors
	d := big.NewInt(3)
	limit := new(big.Int).Sqrt(remaining)
	limit.Add(limit, big.NewInt(1))

	for d.Cmp(limit) <= 0 {
		for new(big.Int).Mod(remaining, d).Cmp(big.NewInt(0)) == 0 {
			factors = append(factors, d.String())
			remaining.Div(remaining, d)
			limit.Sqrt(remaining)
			limit.Add(limit, big.NewInt(1))
		}
		d.Add(d, two)
	}

	if remaining.Cmp(big.NewInt(1)) > 0 {
		factors = append(factors, remaining.String())
	}

	return fmt.Sprintf("%s = %s", n.String(), joinFactors(factors))
}

func joinFactors(factors []string) string {
	if len(factors) == 0 {
		return ""
	}
	result := factors[0]
	for i := 1; i < len(factors); i++ {
		result += " × " + factors[i]
	}
	return result
}
