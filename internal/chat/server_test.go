package chat

import (
	"context"
	"testing"

	"github.com/acai-travel/tech-challenge/internal/chat/model"
	. "github.com/acai-travel/tech-challenge/internal/chat/testing"
	"github.com/acai-travel/tech-challenge/internal/pb"
	"github.com/google/go-cmp/cmp"
	"github.com/twitchtv/twirp"
	"google.golang.org/protobuf/testing/protocmp"
)

// fakeAssistant is a test double that implements the Assistant interface
// with configurable title and reply values.
type fakeAssistant struct {
	title string
	reply string
}

func (f *fakeAssistant) Title(_ context.Context, _ *model.Conversation) (string, error) {
	return f.title, nil
}

func (f *fakeAssistant) Reply(_ context.Context, _ *model.Conversation) (string, error) {
	return f.reply, nil
}

func TestServer_DescribeConversation(t *testing.T) {
	ctx := context.Background()
	srv := NewServer(model.New(ConnectMongo()), nil)

	t.Run("describe existing conversation", WithFixture(func(t *testing.T, f *Fixture) {
		c := f.CreateConversation()

		out, err := srv.DescribeConversation(ctx, &pb.DescribeConversationRequest{ConversationId: c.ID.Hex()})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		got, want := out.GetConversation(), c.Proto()
		if !cmp.Equal(got, want, protocmp.Transform()) {
			t.Errorf("DescribeConversation() mismatch (-got +want):\n%s", cmp.Diff(got, want, protocmp.Transform()))
		}
	}))

	t.Run("describe non existing conversation should return 404", WithFixture(func(t *testing.T, f *Fixture) {
		_, err := srv.DescribeConversation(ctx, &pb.DescribeConversationRequest{ConversationId: "08a59244257c872c5943e2a2"})
		if err == nil {
			t.Fatal("expected error for non-existing conversation, got nil")
		}

		if te, ok := err.(twirp.Error); !ok || te.Code() != twirp.NotFound {
			t.Fatalf("expected twirp.NotFound error, got %v", err)
		}
	}))
}

func TestServer_StartConversation(t *testing.T) {
	ctx := context.Background()
	repo := model.New(ConnectMongo())
	assist := &fakeAssistant{
		title: "Weather Inquiry",
		reply: "It is sunny today!",
	}
	srv := NewServer(repo, assist)

	t.Run("start conversation successfully", WithFixture(func(t *testing.T, f *Fixture) {
		userMessage := "What is the weather like today?"

		out, err := srv.StartConversation(ctx, &pb.StartConversationRequest{Message: userMessage})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		// Verify the response fields.
		if out.GetConversationId() == "" {
			t.Fatal("expected non-empty conversation ID")
		}

		if got, want := out.GetTitle(), "Weather Inquiry"; got != want {
			t.Errorf("title = %q, want %q", got, want)
		}

		if got, want := out.GetReply(), "It is sunny today!"; got != want {
			t.Errorf("reply = %q, want %q", got, want)
		}

		// Verify the conversation was persisted in MongoDB.
		conv, err := repo.DescribeConversation(ctx, out.GetConversationId())
		if err != nil {
			t.Fatalf("failed to describe persisted conversation: %v", err)
		}

		if got, want := conv.Title, "Weather Inquiry"; got != want {
			t.Errorf("persisted title = %q, want %q", got, want)
		}

		// Verify both user and assistant messages are stored.
		if got, want := len(conv.Messages), 2; got != want {
			t.Fatalf("persisted message count = %d, want %d", got, want)
		}

		if got, want := conv.Messages[0].Role, model.RoleUser; got != want {
			t.Errorf("messages[0].Role = %q, want %q", got, want)
		}
		if got, want := conv.Messages[0].Content, userMessage; got != want {
			t.Errorf("messages[0].Content = %q, want %q", got, want)
		}

		if got, want := conv.Messages[1].Role, model.RoleAssistant; got != want {
			t.Errorf("messages[1].Role = %q, want %q", got, want)
		}
		if got, want := conv.Messages[1].Content, "It is sunny today!"; got != want {
			t.Errorf("messages[1].Content = %q, want %q", got, want)
		}

		// Cleanup: delete the conversation created by StartConversation.
		if err := repo.DeleteConversation(ctx, conv.ID.Hex()); err != nil {
			t.Logf("failed to cleanup conversation %s: %v", conv.ID.Hex(), err)
		}
	}))

	t.Run("start conversation with empty message should return error", WithFixture(func(t *testing.T, f *Fixture) {
		_, err := srv.StartConversation(ctx, &pb.StartConversationRequest{Message: ""})
		if err == nil {
			t.Fatal("expected error for empty message, got nil")
		}

		if te, ok := err.(twirp.Error); !ok || te.Code() != twirp.InvalidArgument {
			t.Fatalf("expected twirp.InvalidArgument error, got %v", err)
		}
	}))
}
