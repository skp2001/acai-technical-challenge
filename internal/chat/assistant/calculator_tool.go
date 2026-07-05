package assistant

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/Knetic/govaluate"
	"github.com/openai/openai-go/v2"
)

// CalculatorTool implements the Tool interface for evaluating math expressions.
type CalculatorTool struct{}

func (t *CalculatorTool) Name() string {
	return "calculate"
}

func (t *CalculatorTool) Definition() openai.ChatCompletionToolUnionParam {
	return openai.ChatCompletionFunctionTool(openai.FunctionDefinitionParam{
		Name:        "calculate",
		Description: openai.String("Evaluate simple mathematical expressions."),
		Parameters: openai.FunctionParameters{
			"type": "object",
			"properties": map[string]any{
				"expression": map[string]string{
					"type":        "string",
					"description": "The mathematical expression to evaluate (e.g., '25 * (8 + 4)').",
				},
			},
			"required": []string{"expression"},
		},
	})
}

func (t *CalculatorTool) Execute(ctx context.Context, arguments string) (string, error) {
	var payload struct {
		Expression string `json:"expression"`
	}

	if err := json.Unmarshal([]byte(arguments), &payload); err != nil {
		return "", fmt.Errorf("failed to parse calculate arguments: %w", err)
	}

	expr, err := govaluate.NewEvaluableExpression(payload.Expression)
	if err != nil {
		return "", fmt.Errorf("invalid mathematical expression: %w", err)
	}

	result, err := expr.Evaluate(nil)
	if err != nil {
		return "", fmt.Errorf("failed to evaluate expression: %w", err)
	}

	return fmt.Sprintf("%v", result), nil
}
