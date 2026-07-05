package assistant

import (
	"context"
	"errors"
	"log/slog"
	"strings"

	"github.com/acai-travel/tech-challenge/internal/chat/model"
	"github.com/openai/openai-go/v2"
)

type Assistant struct {
	cli             openai.Client
	tools           map[string]Tool
	toolDefinitions []openai.ChatCompletionToolUnionParam
}

func New() *Assistant {
	toolsList := []Tool{
		&WeatherTool{},
		&TodayDateTool{},
		&HolidaysTool{},
		&CalculatorTool{},
	}

	toolMap := make(map[string]Tool)
	var definitions []openai.ChatCompletionToolUnionParam

	for _, t := range toolsList {
		toolMap[t.Name()] = t
		definitions = append(definitions, t.Definition())
	}

	return &Assistant{
		cli:             openai.NewClient(),
		tools:           toolMap,
		toolDefinitions: definitions,
	}
}

func (a *Assistant) Title(ctx context.Context, conv *model.Conversation) (string, error) {
	if len(conv.Messages) == 0 {
		return "An empty conversation", nil
	}

	slog.InfoContext(ctx, "Generating title for conversation", "conversation_id", conv.ID)

	msgs := make([]openai.ChatCompletionMessageParamUnion, len(conv.Messages)+1)

	msgs[0] = openai.SystemMessage(
		"Generate a concise, descriptive title for the conversation based on the user's first message. Summarize the topic only. Do not answer the question. Return only the title. Maximum 80 characters. No special characters or emojis.",
	)
	for i, m := range conv.Messages {
		msgs[i+1] = openai.UserMessage(m.Content)
	}

	resp, err := a.cli.Chat.Completions.New(ctx, openai.ChatCompletionNewParams{
		Model:    openai.ChatModelO1,
		Messages: msgs,
	})

	if err != nil {
		return "", err
	}

	if len(resp.Choices) == 0 || strings.TrimSpace(resp.Choices[0].Message.Content) == "" {
		return "", errors.New("empty response from OpenAI for title generation")
	}

	title := resp.Choices[0].Message.Content
	title = strings.ReplaceAll(title, "\n", " ")
	title = strings.Trim(title, " \t\r\n-\"'")

	if len(title) > 80 {
		title = title[:80]
	}

	return title, nil
}

func (a *Assistant) Reply(ctx context.Context, conv *model.Conversation) (string, error) {
	if len(conv.Messages) == 0 {
		return "", errors.New("conversation has no messages")
	}

	slog.InfoContext(ctx, "Generating reply for conversation", "conversation_id", conv.ID)

	msgs := []openai.ChatCompletionMessageParamUnion{
		openai.SystemMessage("You are a helpful, concise AI assistant. Provide accurate, safe, and clear responses."),
	}

	for _, m := range conv.Messages {
		switch m.Role {
		case model.RoleUser:
			msgs = append(msgs, openai.UserMessage(m.Content))
		case model.RoleAssistant:
			msgs = append(msgs, openai.AssistantMessage(m.Content))
		}
	}

	for i := 0; i < 15; i++ {
		resp, err := a.cli.Chat.Completions.New(ctx, openai.ChatCompletionNewParams{
			Model:    openai.ChatModelGPT4_1,
			Messages: msgs,
			Tools:    a.toolDefinitions,
		})

		if err != nil {
			return "", err
		}

		if len(resp.Choices) == 0 {
			return "", errors.New("no choices returned by OpenAI")
		}

		if message := resp.Choices[0].Message; len(message.ToolCalls) > 0 {
			msgs = append(msgs, message.ToParam())

			for _, call := range message.ToolCalls {
				slog.InfoContext(ctx, "Tool call received", "name", call.Function.Name, "args", call.Function.Arguments)

				tool, ok := a.tools[call.Function.Name]
				if !ok {
					return "", errors.New("unknown tool call: " + call.Function.Name)
				}

				res, err := tool.Execute(ctx, call.Function.Arguments)
				if err != nil {
					msgs = append(msgs, openai.ToolMessage("failed to execute tool: "+err.Error(), call.ID))
					break
				}
				msgs = append(msgs, openai.ToolMessage(res, call.ID))
			}

			continue
		}

		return resp.Choices[0].Message.Content, nil
	}

	return "", errors.New("too many tool calls, unable to generate reply")
}
