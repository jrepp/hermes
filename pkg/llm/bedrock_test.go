package llm

import (
	"context"
	"strings"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/bedrockruntime"
	"github.com/aws/aws-sdk-go-v2/service/bedrockruntime/types"
	"github.com/hashicorp/go-hclog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/hashicorp-forge/hermes/pkg/indexer/pipeline/steps"
)

// MockBedrockClient mocks the Bedrock API for testing
type MockBedrockClient struct {
	ConverseFunc func(ctx context.Context, params *bedrockruntime.ConverseInput, optFns ...func(*bedrockruntime.Options)) (*bedrockruntime.ConverseOutput, error)
}

func (m *MockBedrockClient) Converse(ctx context.Context, params *bedrockruntime.ConverseInput, optFns ...func(*bedrockruntime.Options)) (*bedrockruntime.ConverseOutput, error) {
	return m.ConverseFunc(ctx, params, optFns...)
}

func TestBedrockClient_GenerateSummary(t *testing.T) {
	ctx := context.Background()

	mockClient := &MockBedrockClient{
		ConverseFunc: func(ctx context.Context, params *bedrockruntime.ConverseInput, optFns ...func(*bedrockruntime.Options)) (*bedrockruntime.ConverseOutput, error) {
			// Verify request
			require.NotNil(t, params.ModelId)
			assert.Equal(t, "us.anthropic.claude-3-7-sonnet-20250219-v1:0", *params.ModelId)
			require.NotNil(t, params.InferenceConfig)
			assert.Equal(t, int32(500), *params.InferenceConfig.MaxTokens)
			assert.Equal(t, float32(0.3), *params.InferenceConfig.Temperature)

			// Return mock response
			return &bedrockruntime.ConverseOutput{
				Output: &types.ConverseOutputMemberMessage{
					Value: types.Message{
						Role: types.ConversationRoleAssistant,
						Content: []types.ContentBlock{
							&types.ContentBlockMemberText{
								Value: "EXECUTIVE SUMMARY:\nThis is a comprehensive test document that covers important topics related to software architecture and best practices.\n\nKEY POINTS:\n- The document emphasizes scalable design patterns\n- Performance optimization is a key consideration\n- Security measures are thoroughly discussed\n\nTOPICS:\nsoftware architecture, scalability, performance, security\n\nTAGS:\narchitecture, best-practices, engineering",
							},
						},
					},
				},
				Usage: &types.TokenUsage{
					TotalTokens: aws.Int32(250),
				},
			}, nil
		},
	}

	client := &BedrockClient{
		client: mockClient,
		logger: hclog.NewNullLogger(),
	}

	// Test summary generation
	summary, err := client.GenerateSummary(ctx, "This is a test document content", steps.SummaryOptions{
		Model:     "us.anthropic.claude-3-7-sonnet-20250219-v1:0",
		MaxTokens: 500,
		Language:  "en",
		Style:     "executive",
	})

	require.NoError(t, err)
	require.NotNil(t, summary)

	// Verify summary content
	assert.Contains(t, summary.ExecutiveSummary, "comprehensive test document")
	assert.Len(t, summary.KeyPoints, 3)
	assert.Contains(t, summary.KeyPoints[0], "scalable design patterns")
	assert.Len(t, summary.Topics, 4)
	assert.Contains(t, summary.Topics, "software architecture")
	assert.Len(t, summary.Tags, 3)
	assert.Contains(t, summary.Tags, "architecture")
	assert.Equal(t, 250, summary.TokensUsed)
	assert.GreaterOrEqual(t, summary.GenerationTimeMs, 0)
}

func TestBedrockClient_GenerateSummary_DefaultModel(t *testing.T) {
	ctx := context.Background()

	mockClient := &MockBedrockClient{
		ConverseFunc: func(ctx context.Context, params *bedrockruntime.ConverseInput, optFns ...func(*bedrockruntime.Options)) (*bedrockruntime.ConverseOutput, error) {
			// Verify default model is used
			require.NotNil(t, params.ModelId)
			assert.Equal(t, "us.anthropic.claude-3-7-sonnet-20250219-v1:0", *params.ModelId)

			return &bedrockruntime.ConverseOutput{
				Output: &types.ConverseOutputMemberMessage{
					Value: types.Message{
						Role: types.ConversationRoleAssistant,
						Content: []types.ContentBlock{
							&types.ContentBlockMemberText{
								Value: "EXECUTIVE SUMMARY:\nTest summary.\n\nKEY POINTS:\n- Point 1\n\nTOPICS:\ntopic1\n\nTAGS:\ntag1",
							},
						},
					},
				},
			}, nil
		},
	}

	client := &BedrockClient{
		client: mockClient,
		logger: hclog.NewNullLogger(),
	}

	// Don't specify model - should use default
	summary, err := client.GenerateSummary(ctx, "Test content", steps.SummaryOptions{
		MaxTokens: 500,
	})

	require.NoError(t, err)
	require.NotNil(t, summary)
}

func TestBedrockClient_GenerateSummary_EmptyResponse(t *testing.T) {
	ctx := context.Background()

	mockClient := &MockBedrockClient{
		ConverseFunc: func(ctx context.Context, params *bedrockruntime.ConverseInput, optFns ...func(*bedrockruntime.Options)) (*bedrockruntime.ConverseOutput, error) {
			return &bedrockruntime.ConverseOutput{
				Output: &types.ConverseOutputMemberMessage{
					Value: types.Message{
						Role:    types.ConversationRoleAssistant,
						Content: []types.ContentBlock{}, // Empty content
					},
				},
			}, nil
		},
	}

	client := &BedrockClient{
		client: mockClient,
		logger: hclog.NewNullLogger(),
	}

	summary, err := client.GenerateSummary(ctx, "Test content", steps.SummaryOptions{
		MaxTokens: 500,
	})

	require.Error(t, err)
	assert.Nil(t, summary)
	assert.Contains(t, err.Error(), "no message content in Bedrock response")
}

func TestBedrockClient_GenerateSummary_NoOutput(t *testing.T) {
	ctx := context.Background()

	mockClient := &MockBedrockClient{
		ConverseFunc: func(ctx context.Context, params *bedrockruntime.ConverseInput, optFns ...func(*bedrockruntime.Options)) (*bedrockruntime.ConverseOutput, error) {
			return &bedrockruntime.ConverseOutput{
				Output: nil, // No output
			}, nil
		},
	}

	client := &BedrockClient{
		client: mockClient,
		logger: hclog.NewNullLogger(),
	}

	summary, err := client.GenerateSummary(ctx, "Test content", steps.SummaryOptions{
		MaxTokens: 500,
	})

	require.Error(t, err)
	assert.Nil(t, summary)
	assert.Contains(t, err.Error(), "no output in Bedrock response")
}

func TestBedrockClient_ParseSummaryResponse(t *testing.T) {
	client := &BedrockClient{
		logger: hclog.NewNullLogger(),
	}

	tests := []struct {
		name     string
		content  string
		wantErr  bool
		validate func(t *testing.T, summary *steps.Summary)
	}{
		{
			name: "valid response",
			content: `EXECUTIVE SUMMARY:
This document provides guidance on API design principles and best practices.

KEY POINTS:
- RESTful API design patterns
- Authentication and authorization strategies
- Rate limiting and throttling

TOPICS:
API design, REST, security, performance

TAGS:
api, rest, security, design`,
			wantErr: false,
			validate: func(t *testing.T, summary *steps.Summary) {
				assert.Contains(t, summary.ExecutiveSummary, "API design principles")
				assert.Len(t, summary.KeyPoints, 3)
				assert.Equal(t, "RESTful API design patterns", summary.KeyPoints[0])
				assert.Len(t, summary.Topics, 4)
				assert.Len(t, summary.Tags, 4)
			},
		},
		{
			name: "missing executive summary",
			content: `KEY POINTS:
- Point one
- Point two

TOPICS:
topic1, topic2`,
			wantErr: true,
		},
		{
			name:    "exact mock response content",
			content: "EXECUTIVE SUMMARY:\nThis is a comprehensive test document that covers important topics related to software architecture and best practices.\n\nKEY POINTS:\n- The document emphasizes scalable design patterns\n- Performance optimization is a key consideration\n- Security measures are thoroughly discussed\n\nTOPICS:\nsoftware architecture, scalability, performance, security\n\nTAGS:\narchitecture, best-practices, engineering",
			wantErr: false,
			validate: func(t *testing.T, summary *steps.Summary) {
				assert.Contains(t, summary.ExecutiveSummary, "comprehensive test document")
				assert.Len(t, summary.KeyPoints, 3)
				assert.Len(t, summary.Topics, 4)
				assert.Len(t, summary.Tags, 3)
			},
		},
		{
			name: "alternative bullet formats",
			content: `EXECUTIVE SUMMARY:
Test summary here.

KEY POINTS:
* First point with asterisk
• Second point with bullet
- Third point with dash

TOPICS:
topic1, topic2

TAGS:
tag1, tag2`,
			wantErr: false,
			validate: func(t *testing.T, summary *steps.Summary) {
				assert.Len(t, summary.KeyPoints, 3)
				assert.Contains(t, summary.KeyPoints[0], "First point")
				assert.Contains(t, summary.KeyPoints[1], "Second point")
				assert.Contains(t, summary.KeyPoints[2], "Third point")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			summary, err := client.parseSummaryResponse(tt.content)

			if tt.wantErr {
				require.Error(t, err)
				return
			}

			require.NoError(t, err)
			require.NotNil(t, summary)

			if tt.validate != nil {
				tt.validate(t, summary)
			}
		})
	}
}

func TestBedrockClient_ContentTruncation(t *testing.T) {
	client := &BedrockClient{
		logger: hclog.NewNullLogger(),
	}

	// Create very long content
	longContent := string(make([]byte, 50000))

	options := steps.SummaryOptions{
		MaxTokens: 500,
		Style:     "executive",
	}

	prompt := client.buildPrompt(longContent, options)

	// Verify content was truncated
	assert.Contains(t, prompt, "[Content truncated...]")
	assert.Less(t, len(prompt), 50000)
}

func TestBedrockClient_BuildPrompt_DifferentStyles(t *testing.T) {
	client := &BedrockClient{
		logger: hclog.NewNullLogger(),
	}

	styles := map[string]string{
		"executive":     "executive summary suitable for leadership",
		"technical":     "technical summary with implementation details",
		"bullet-points": "concise bullet points",
		"default":       "clear and comprehensive summary",
	}

	for style, expectedPhrase := range styles {
		t.Run(style, func(t *testing.T) {
			options := steps.SummaryOptions{
				Style: style,
			}

			prompt := client.buildPrompt("Test content", options)
			assert.Contains(t, strings.ToLower(prompt), strings.ToLower(expectedPhrase))
		})
	}
}

func TestBedrockClient_SystemPrompt(t *testing.T) {
	client := &BedrockClient{
		logger: hclog.NewNullLogger(),
	}

	options := steps.SummaryOptions{}
	systemPrompt := client.getSystemPrompt(options)

	// Verify system prompt contains key instructions
	assert.Contains(t, systemPrompt, "EXECUTIVE SUMMARY")
	assert.Contains(t, systemPrompt, "KEY POINTS")
	assert.Contains(t, systemPrompt, "TOPICS")
	assert.Contains(t, systemPrompt, "TAGS")
	assert.Contains(t, systemPrompt, "expert document analyst")
}

func TestBedrockClient_DifferentModels(t *testing.T) {
	ctx := context.Background()

	models := []string{
		"us.anthropic.claude-3-7-sonnet-20250219-v1:0",
		"us.anthropic.claude-3-5-sonnet-20241022-v2:0",
		"anthropic.claude-3-opus-20240229-v1:0",
	}

	for _, model := range models {
		t.Run(model, func(t *testing.T) {
			mockClient := &MockBedrockClient{
				ConverseFunc: func(ctx context.Context, params *bedrockruntime.ConverseInput, optFns ...func(*bedrockruntime.Options)) (*bedrockruntime.ConverseOutput, error) {
					assert.Equal(t, model, *params.ModelId)

					return &bedrockruntime.ConverseOutput{
						Output: &types.ConverseOutputMemberMessage{
							Value: types.Message{
								Content: []types.ContentBlock{
									&types.ContentBlockMemberText{
										Value: "EXECUTIVE SUMMARY:\nTest summary.\n\nKEY POINTS:\n- Point 1\n\nTOPICS:\ntopic1\n\nTAGS:\ntag1",
									},
								},
							},
						},
					}, nil
				},
			}

			client := &BedrockClient{
				client: mockClient,
				logger: hclog.NewNullLogger(),
			}

			summary, err := client.GenerateSummary(ctx, "test", steps.SummaryOptions{
				Model: model,
			})

			require.NoError(t, err)
			assert.NotNil(t, summary)
		})
	}
}
