package assistant

import (
	"context"

	"github.com/openai/openai-go/v2"
)

// Tool represents an capability that the AI assistant can invoke.
type Tool interface {
	// Name returns the unique identifier for this tool.
	// This matches the tool name returned by the OpenAI API in a tool call.
	Name() string

	// Definition returns the OpenAI function definition schema.
	// This is passed to the LLM so it knows how to call the tool and what parameters it accepts.
	Definition() openai.ChatCompletionToolUnionParam

	// Execute runs the tool's business logic using the arguments provided by the LLM.
	// The arguments are passed as a raw JSON string and must be parsed by the tool implementation.
	// It returns a string representing the result of the tool execution, which will be sent back to the LLM.
	Execute(ctx context.Context, arguments string) (string, error)
}
