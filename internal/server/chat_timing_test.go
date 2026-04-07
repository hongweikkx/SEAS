package server

import (
	"bytes"
	"context"
	"errors"
	"io"
	"strings"
	"testing"

	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"
	"github.com/go-kratos/kratos/v2/log"
)

type fakeChatModel struct {
	generateFn func(context.Context, []*schema.Message, ...model.Option) (*schema.Message, error)
	streamFn   func(context.Context, []*schema.Message, ...model.Option) (*schema.StreamReader[*schema.Message], error)
}

func (f *fakeChatModel) Generate(ctx context.Context, input []*schema.Message, opts ...model.Option) (*schema.Message, error) {
	if f.generateFn != nil {
		return f.generateFn(ctx, input, opts...)
	}
	return schema.AssistantMessage("ok", nil), nil
}

func (f *fakeChatModel) Stream(ctx context.Context, input []*schema.Message, opts ...model.Option) (*schema.StreamReader[*schema.Message], error) {
	if f.streamFn != nil {
		return f.streamFn(ctx, input, opts...)
	}

	sr, sw := schema.Pipe[*schema.Message](1)
	go func() {
		defer sw.Close()
		_ = sw.Send(schema.AssistantMessage("ok", nil), nil)
	}()
	return sr, nil
}

func TestChatTimingMiddlewareGenerateLogsDuration(t *testing.T) {
	var buf bytes.Buffer
	helper := log.NewHelper(log.NewStdLogger(&buf))

	mw := newChatTimingMiddleware(helper, "ark-test-model")
	wrapped, err := mw.WrapModel(context.Background(), &fakeChatModel{
		generateFn: func(context.Context, []*schema.Message, ...model.Option) (*schema.Message, error) {
			return schema.AssistantMessage("hello", nil), nil
		},
	}, nil)
	if err != nil {
		t.Fatalf("WrapModel() error = %v", err)
	}

	msg, err := wrapped.Generate(context.Background(), []*schema.Message{schema.UserMessage("hi")})
	if err != nil {
		t.Fatalf("Generate() error = %v", err)
	}
	if msg == nil || msg.Content != "hello" {
		t.Fatalf("unexpected message: %#v", msg)
	}

	out := buf.String()
	if !strings.Contains(out, "llm call finished") {
		t.Fatalf("expected log to contain finish message, got %q", out)
	}
	if !strings.Contains(out, "model") || !strings.Contains(out, "ark-test-model") {
		t.Fatalf("expected log to contain model name, got %q", out)
	}
	if !strings.Contains(out, "total_ms") {
		t.Fatalf("expected log to contain total_ms, got %q", out)
	}
}

func TestChatTimingMiddlewareStreamLogsDuration(t *testing.T) {
	var buf bytes.Buffer
	helper := log.NewHelper(log.NewStdLogger(&buf))

	mw := newChatTimingMiddleware(helper, "ark-test-model")
	wrapped, err := mw.WrapModel(context.Background(), &fakeChatModel{
		streamFn: func(context.Context, []*schema.Message, ...model.Option) (*schema.StreamReader[*schema.Message], error) {
			sr, sw := schema.Pipe[*schema.Message](1)
			go func() {
				defer sw.Close()
				_ = sw.Send(schema.AssistantMessage("part-1", nil), nil)
				_ = sw.Send(schema.AssistantMessage("part-2", nil), nil)
			}()
			return sr, nil
		},
	}, nil)
	if err != nil {
		t.Fatalf("WrapModel() error = %v", err)
	}

	stream, err := wrapped.Stream(context.Background(), []*schema.Message{schema.UserMessage("hi")})
	if err != nil {
		t.Fatalf("Stream() error = %v", err)
	}
	defer stream.Close()

	var contents []string
	for {
		msg, recvErr := stream.Recv()
		if errors.Is(recvErr, io.EOF) {
			break
		}
		if recvErr != nil {
			t.Fatalf("Recv() error = %v", recvErr)
		}
		contents = append(contents, msg.Content)
	}

	if got := strings.Join(contents, ","); got != "part-1,part-2" {
		t.Fatalf("unexpected stream contents: %q", got)
	}

	out := buf.String()
	if !strings.Contains(out, "llm call finished") {
		t.Fatalf("expected log to contain finish message, got %q", out)
	}
	if !strings.Contains(out, "llm first token") {
		t.Fatalf("expected log to contain first token message, got %q", out)
	}
	if !strings.Contains(out, "mode") || !strings.Contains(out, "stream") {
		t.Fatalf("expected log to contain stream mode, got %q", out)
	}
	if !strings.Contains(out, "first_token_ms") || !strings.Contains(out, "total_ms") {
		t.Fatalf("expected log to contain first_token_ms and total_ms, got %q", out)
	}
}
