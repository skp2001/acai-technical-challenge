package assistant

import (
	"context"
	"time"

	"github.com/openai/openai-go/v2"
)

// TodayDateTool implements the Tool interface for getting the current date and time.
type TodayDateTool struct{}

func (t *TodayDateTool) Name() string {
	return "get_today_date"
}

func (t *TodayDateTool) Definition() openai.ChatCompletionToolUnionParam {
	return openai.ChatCompletionFunctionTool(openai.FunctionDefinitionParam{
		Name:        "get_today_date",
		Description: openai.String("Get today's date and time in RFC3339 format"),
	})
}

func (t *TodayDateTool) Execute(ctx context.Context, arguments string) (string, error) {
	return time.Now().Format(time.RFC3339), nil
}
