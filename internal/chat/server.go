package chat

import (
	"context"
	"log/slog"
	"strings"
	"time"

	"github.com/acai-travel/tech-challenge/internal/chat/model"
	"github.com/acai-travel/tech-challenge/internal/pb"
	"github.com/twitchtv/twirp"
	"go.mongodb.org/mongo-driver/bson/primitive"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/metric"
)

var (
	meter          = otel.GetMeterProvider().Meter("acai-chat")
	tracer         = otel.Tracer("acai-chat")
	requestCounter = func() metric.Int64Counter {
		c, _ := meter.Int64Counter(
			"rpc_requests_total",
			metric.WithDescription("Total number of RPC requests"),
		)
		return c
	}()
	latencyHistogram = func() metric.Float64Histogram {
		h, _ := meter.Float64Histogram(
			"rpc_latency_ms",
			metric.WithDescription("RPC latency in milliseconds"),
		)
		return h
	}()
	errorCounter = func() metric.Int64Counter {
		c, _ := meter.Int64Counter(
			"rpc_errors_total",
			metric.WithDescription("Total number of RPC errors"),
		)
		return c
	}()
)

var _ pb.ChatService = (*Server)(nil)

type Assistant interface {
	Title(ctx context.Context, conv *model.Conversation) (string, error)
	Reply(ctx context.Context, conv *model.Conversation) (string, error)
}

type Server struct {
	repo   *model.Repository
	assist Assistant
}

func NewServer(repo *model.Repository, assist Assistant) *Server {
	return &Server{repo: repo, assist: assist}
}

func (s *Server) StartConversation(ctx context.Context, req *pb.StartConversationRequest) (*pb.StartConversationResponse, error) {
	start := time.Now()
	ctx, span := tracer.Start(ctx, "StartConversation")
	defer span.End()
	span.SetAttributes(attribute.String("rpc.method", "StartConversation"))
	requestCounter.Add(ctx, 1, metric.WithAttributes(attribute.String("method", "StartConversation")))

	if strings.TrimSpace(req.GetMessage()) == "" {
		err := twirp.RequiredArgumentError("message")
		errorCounter.Add(ctx, 1, metric.WithAttributes(attribute.String("method", "StartConversation")))
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		latencyHistogram.Record(ctx, float64(time.Since(start).Milliseconds()), metric.WithAttributes(attribute.String("method", "StartConversation")))
		return nil, err
	}

	conversation := &model.Conversation{
		ID:        primitive.NewObjectID(),
		Title:     "Untitled conversation",
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
		Messages: []*model.Message{{
			ID:        primitive.NewObjectID(),
			Role:      model.RoleUser,
			Content:   req.GetMessage(),
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		}},
	}

	// choose a title
	title, err := s.assist.Title(ctx, conversation)
	if err != nil {
		slog.ErrorContext(ctx, "Failed to generate conversation title", "error", err)
	} else {
		conversation.Title = title
	}

	// generate a reply
	reply, err := s.assist.Reply(ctx, conversation)
	if err != nil {
		errorCounter.Add(ctx, 1, metric.WithAttributes(attribute.String("method", "StartConversation")))
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		latencyHistogram.Record(ctx, float64(time.Since(start).Milliseconds()), metric.WithAttributes(attribute.String("method", "StartConversation")))
		return nil, err
	}

	conversation.Messages = append(conversation.Messages, &model.Message{
		ID:        primitive.NewObjectID(),
		Role:      model.RoleAssistant,
		Content:   reply,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	})

	if err := s.repo.CreateConversation(ctx, conversation); err != nil {
		errorCounter.Add(ctx, 1, metric.WithAttributes(attribute.String("method", "StartConversation")))
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		latencyHistogram.Record(ctx, float64(time.Since(start).Milliseconds()), metric.WithAttributes(attribute.String("method", "StartConversation")))
		return nil, err
	}

	span.SetAttributes(attribute.String("conversation.id", conversation.ID.Hex()))
	span.SetStatus(codes.Ok, "")
	latencyHistogram.Record(ctx, float64(time.Since(start).Milliseconds()), metric.WithAttributes(attribute.String("method", "StartConversation")))

	return &pb.StartConversationResponse{
		ConversationId: conversation.ID.Hex(),
		Title:          conversation.Title,
		Reply:          reply,
	}, nil
}

func (s *Server) ContinueConversation(ctx context.Context, req *pb.ContinueConversationRequest) (*pb.ContinueConversationResponse, error) {
	start := time.Now()
	ctx, span := tracer.Start(ctx, "ContinueConversation")
	defer span.End()
	span.SetAttributes(attribute.String("rpc.method", "ContinueConversation"))
	requestCounter.Add(ctx, 1, metric.WithAttributes(attribute.String("method", "ContinueConversation")))

	if req.GetConversationId() == "" {
		err := twirp.RequiredArgumentError("conversation_id")
		errorCounter.Add(ctx, 1, metric.WithAttributes(attribute.String("method", "ContinueConversation")))
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		latencyHistogram.Record(ctx, float64(time.Since(start).Milliseconds()), metric.WithAttributes(attribute.String("method", "ContinueConversation")))
		return nil, err
	}
	if strings.TrimSpace(req.GetMessage()) == "" {
		err := twirp.RequiredArgumentError("message")
		errorCounter.Add(ctx, 1, metric.WithAttributes(attribute.String("method", "ContinueConversation")))
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		latencyHistogram.Record(ctx, float64(time.Since(start).Milliseconds()), metric.WithAttributes(attribute.String("method", "ContinueConversation")))
		return nil, err
	}

	conversation, err := s.repo.DescribeConversation(ctx, req.GetConversationId())
	if err != nil {
		errorCounter.Add(ctx, 1, metric.WithAttributes(attribute.String("method", "ContinueConversation")))
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		latencyHistogram.Record(ctx, float64(time.Since(start).Milliseconds()), metric.WithAttributes(attribute.String("method", "ContinueConversation")))
		return nil, err
	}

	conversation.UpdatedAt = time.Now()
	conversation.Messages = append(conversation.Messages, &model.Message{
		ID:        primitive.NewObjectID(),
		Role:      model.RoleUser,
		Content:   req.GetMessage(),
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	})

	reply, err := s.assist.Reply(ctx, conversation)
	if err != nil {
		errorCounter.Add(ctx, 1, metric.WithAttributes(attribute.String("method", "ContinueConversation")))
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		latencyHistogram.Record(ctx, float64(time.Since(start).Milliseconds()), metric.WithAttributes(attribute.String("method", "ContinueConversation")))
		return nil, twirp.InternalErrorWith(err)
	}

	conversation.Messages = append(conversation.Messages, &model.Message{
		ID:        primitive.NewObjectID(),
		Role:      model.RoleAssistant,
		Content:   reply,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	})

	if err := s.repo.UpdateConversation(ctx, conversation); err != nil {
		errorCounter.Add(ctx, 1, metric.WithAttributes(attribute.String("method", "ContinueConversation")))
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		latencyHistogram.Record(ctx, float64(time.Since(start).Milliseconds()), metric.WithAttributes(attribute.String("method", "ContinueConversation")))
		return nil, twirp.InternalErrorWith(err)
	}

	span.SetAttributes(attribute.String("conversation.id", conversation.ID.Hex()))
	span.SetStatus(codes.Ok, "")
	latencyHistogram.Record(ctx, float64(time.Since(start).Milliseconds()), metric.WithAttributes(attribute.String("method", "ContinueConversation")))

	return &pb.ContinueConversationResponse{Reply: reply}, nil
}

func (s *Server) ListConversations(ctx context.Context, req *pb.ListConversationsRequest) (*pb.ListConversationsResponse, error) {
	start := time.Now()
	ctx, span := tracer.Start(ctx, "ListConversations")
	defer span.End()
	span.SetAttributes(attribute.String("rpc.method", "ListConversations"))
	requestCounter.Add(ctx, 1, metric.WithAttributes(attribute.String("method", "ListConversations")))

	conversations, err := s.repo.ListConversations(ctx)
	if err != nil {
		errorCounter.Add(ctx, 1, metric.WithAttributes(attribute.String("method", "ListConversations")))
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		latencyHistogram.Record(ctx, float64(time.Since(start).Milliseconds()), metric.WithAttributes(attribute.String("method", "ListConversations")))
		return nil, twirp.InternalErrorWith(err)
	}

	resp := &pb.ListConversationsResponse{}
	for _, conv := range conversations {
		conv.Messages = nil
		resp.Conversations = append(resp.Conversations, conv.Proto())
	}

	span.SetStatus(codes.Ok, "")
	latencyHistogram.Record(ctx, float64(time.Since(start).Milliseconds()), metric.WithAttributes(attribute.String("method", "ListConversations")))
	return resp, nil
}

func (s *Server) DescribeConversation(ctx context.Context, req *pb.DescribeConversationRequest) (*pb.DescribeConversationResponse, error) {
	start := time.Now()
	ctx, span := tracer.Start(ctx, "DescribeConversation")
	defer span.End()
	span.SetAttributes(attribute.String("rpc.method", "DescribeConversation"))
	requestCounter.Add(ctx, 1, metric.WithAttributes(attribute.String("method", "DescribeConversation")))

	if req.GetConversationId() == "" {
		err := twirp.RequiredArgumentError("conversation_id")
		errorCounter.Add(ctx, 1, metric.WithAttributes(attribute.String("method", "DescribeConversation")))
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		latencyHistogram.Record(ctx, float64(time.Since(start).Milliseconds()), metric.WithAttributes(attribute.String("method", "DescribeConversation")))
		return nil, err
	}

	conversation, err := s.repo.DescribeConversation(ctx, req.GetConversationId())
	if err != nil {
		errorCounter.Add(ctx, 1, metric.WithAttributes(attribute.String("method", "DescribeConversation")))
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		latencyHistogram.Record(ctx, float64(time.Since(start).Milliseconds()), metric.WithAttributes(attribute.String("method", "DescribeConversation")))
		return nil, err
	}
	if conversation == nil {
		err := twirp.NotFoundError("conversation not found")
		errorCounter.Add(ctx, 1, metric.WithAttributes(attribute.String("method", "DescribeConversation")))
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		latencyHistogram.Record(ctx, float64(time.Since(start).Milliseconds()), metric.WithAttributes(attribute.String("method", "DescribeConversation")))
		return nil, err
	}

	span.SetAttributes(attribute.String("conversation.id", conversation.ID.Hex()))
	span.SetStatus(codes.Ok, "")
	latencyHistogram.Record(ctx, float64(time.Since(start).Milliseconds()), metric.WithAttributes(attribute.String("method", "DescribeConversation")))

	return &pb.DescribeConversationResponse{Conversation: conversation.Proto()}, nil
}
