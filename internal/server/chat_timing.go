package server

import (
	"context"
	"fmt"
	"io"
	"time"

	"github.com/cloudwego/eino/adk"
	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"
	"github.com/go-kratos/kratos/v2/log"
)

type chatTimingMiddleware struct {
	*adk.BaseChatModelAgentMiddleware

	logger    *log.Helper
	modelName string
}

type timedChatModel struct {
	inner     model.BaseChatModel
	logger    *log.Helper
	modelName string
}

func newChatTimingMiddleware(logger *log.Helper, modelName string) adk.ChatModelAgentMiddleware {
	return &chatTimingMiddleware{
		BaseChatModelAgentMiddleware: &adk.BaseChatModelAgentMiddleware{},
		logger:                       logger,
		modelName:                    modelName,
	}
}

func (m *chatTimingMiddleware) WrapModel(_ context.Context, cm model.BaseChatModel, _ *adk.ModelContext) (model.BaseChatModel, error) {
	if cm == nil {
		return nil, fmt.Errorf("chat model is nil")
	}
	return &timedChatModel{
		inner:     cm,
		logger:    m.logger,
		modelName: m.modelName,
	}, nil
}

func (m *timedChatModel) Generate(ctx context.Context, input []*schema.Message, opts ...model.Option) (*schema.Message, error) {
	start := time.Now()
	msg, err := m.inner.Generate(ctx, input, opts...)
	m.logFinished("generate", time.Since(start), 0, false, err)
	return msg, err
}

func (m *timedChatModel) Stream(ctx context.Context, input []*schema.Message, opts ...model.Option) (*schema.StreamReader[*schema.Message], error) {
	start := time.Now()
	stream, err := m.inner.Stream(ctx, input, opts...)
	if err != nil {
		m.logFinished("stream", time.Since(start), 0, false, err)
		return nil, err
	}
	if stream == nil {
		m.logFinished("stream", time.Since(start), 0, false, nil)
		return nil, nil
	}

	out, sw := schema.Pipe[*schema.Message](16)
	go func() {
		defer stream.Close()
		defer sw.Close()

		var callErr error
		var firstToken time.Duration
		var firstTokenSeen bool
		defer func() {
			m.logFinished("stream", time.Since(start), firstToken, firstTokenSeen, callErr)
		}()

		for {
			chunk, recvErr := stream.Recv()
			if recvErr != nil {
				if recvErr == io.EOF {
					return
				}
				callErr = recvErr
				_ = sw.Send(nil, recvErr)
				return
			}
			if !firstTokenSeen {
				firstTokenSeen = true
				firstToken = time.Since(start)
				m.logFirstToken("stream", firstToken)
			}
			if closed := sw.Send(chunk, nil); closed {
				return
			}
		}
	}()

	return out, nil
}

func (m *timedChatModel) logFirstToken(mode string, firstToken time.Duration) {
	if m.logger == nil {
		return
	}
	m.logger.Infof("llm first token: model=%s mode=%s first_token_ms=%d", m.modelName, mode, firstToken.Milliseconds())
}

func (m *timedChatModel) logFinished(mode string, total time.Duration, firstToken time.Duration, hasFirstToken bool, err error) {
	if m.logger == nil {
		return
	}
	if err != nil {
		if hasFirstToken {
			m.logger.Warnf("llm call finished: model=%s mode=%s first_token_ms=%d total_ms=%d error=%s", m.modelName, mode, firstToken.Milliseconds(), total.Milliseconds(), err.Error())
			return
		}
		m.logger.Warnf("llm call finished: model=%s mode=%s total_ms=%d error=%s", m.modelName, mode, total.Milliseconds(), err.Error())
		return
	}
	if hasFirstToken {
		m.logger.Infof("llm call finished: model=%s mode=%s first_token_ms=%d total_ms=%d", m.modelName, mode, firstToken.Milliseconds(), total.Milliseconds())
		return
	}
	m.logger.Infof("llm call finished: model=%s mode=%s total_ms=%d", m.modelName, mode, total.Milliseconds())
}
