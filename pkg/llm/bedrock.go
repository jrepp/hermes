package llm

import (
	"context"
	"fmt"
	"math"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/bedrockruntime"
	"github.com/aws/aws-sdk-go-v2/service/bedrockruntime/types"
	"github.com/hashicorp/go-hclog"

	"github.com/hashicorp-forge/hermes/pkg/indexer/pipeline/steps"
)

// safeIntToInt32 converts int to int32, checking for overflow.
// Returns math.MaxInt32 if the value would overflow.
func safeIntToInt32(i int) int32 {
	if i > math.MaxInt32 {
		return math.MaxInt32
	}
	if i < math.MinInt32 {
		return math.MinInt32
	}
	return int32(i)
}

// BedrockConverseAPI defines the interface for Bedrock Converse operations.
// This allows for testing with mocks.
type BedrockConverseAPI interface {
	Converse(ctx context.Context, params *bedrockruntime.ConverseInput, optFns ...func(*bedrockruntime.Options)) (*bedrockruntime.ConverseOutput, error)
}

// BedrockClient implements the LLMClient interface for AWS Bedrock Converse API.
type BedrockClient struct {
	client BedrockConverseAPI
	logger hclog.Logger
}

// BedrockConfig holds configuration for the Bedrock client.
type BedrockConfig struct {
	Logger hclog.Logger
	Region string
}

// NewBedrockClient creates a new AWS Bedrock client using the Converse API.
func NewBedrockClient(ctx context.Context, cfg BedrockConfig) (*BedrockClient, error) {
	if cfg.Region == "" {
		cfg.Region = "us-east-1"
	}

	if cfg.Logger == nil {
		cfg.Logger = hclog.NewNullLogger()
	}

	// Load AWS config
	awsCfg, err := config.LoadDefaultConfig(ctx, config.WithRegion(cfg.Region))
	if err != nil {
		return nil, fmt.Errorf("failed to load AWS config: %w", err)
	}

	return &BedrockClient{
		client: bedrockruntime.NewFromConfig(awsCfg),
		logger: cfg.Logger.Named("bedrock-client"),
	}, nil
}

// GenerateSummary generates a summary using AWS Bedrock's Converse API.
func (c *BedrockClient) GenerateSummary(ctx context.Context, content string, options steps.SummaryOptions) (*steps.Summary, error) {
	startTime := time.Now()

	// Use default model if not specified
	model := options.Model
	if model == "" {
		model = "us.anthropic.claude-3-7-sonnet-20250219-v1:0"
	}

	// Build the prompt
	prompt := c.buildPrompt(content, options)

	// Prepare the Converse API request
	input := &bedrockruntime.ConverseInput{
		ModelId: aws.String(model),
		Messages: []types.Message{
			{
				Role: types.ConversationRoleUser,
				Content: []types.ContentBlock{
					&types.ContentBlockMemberText{
						Value: prompt,
					},
				},
			},
		},
		System: []types.SystemContentBlock{
			&types.SystemContentBlockMemberText{
				Value: c.getSystemPrompt(options),
			},
		},
		InferenceConfig: &types.InferenceConfiguration{
			MaxTokens:   aws.Int32(safeIntToInt32(options.MaxTokens)),
			Temperature: aws.Float32(0.3), // Lower temperature for consistent summaries
		},
	}

	c.logger.Debug("sending request to Bedrock",
		"model", model,
		"max_tokens", options.MaxTokens,
		"content_length", len(content),
	)

	// Send request
	resp, err := c.client.Converse(ctx, input)
	if err != nil {
		return nil, fmt.Errorf("failed to call Bedrock Converse API: %w", err)
	}

	// Extract text from response
	if resp.Output == nil {
		return nil, fmt.Errorf("no output in Bedrock response")
	}

	message, ok := resp.Output.(*types.ConverseOutputMemberMessage)
	if !ok || message == nil || len(message.Value.Content) == 0 {
		return nil, fmt.Errorf("no message content in Bedrock response")
	}

	// Extract text from first content block
	var responseText string
	for _, block := range message.Value.Content {
		if textBlock, ok := block.(*types.ContentBlockMemberText); ok {
			responseText = textBlock.Value
			break
		}
	}

	if responseText == "" {
		return nil, fmt.Errorf("empty response from Bedrock")
	}

	generationTime := int(time.Since(startTime).Milliseconds())

	// Parse the LLM response into structured summary
	summary, err := c.parseSummaryResponse(responseText)
	if err != nil {
		return nil, fmt.Errorf("failed to parse summary: %w", err)
	}

	// Add metadata
	if resp.Usage != nil && resp.Usage.TotalTokens != nil {
		summary.TokensUsed = int(*resp.Usage.TotalTokens)
	}
	summary.GenerationTimeMs = generationTime

	c.logger.Info("generated summary via Bedrock",
		"model", model,
		"tokens_used", summary.TokensUsed,
		"generation_time_ms", generationTime,
	)

	return summary, nil
}

// buildPrompt builds the prompt for summary generation.
func (c *BedrockClient) buildPrompt(content string, options steps.SummaryOptions) string {
	return buildSummaryPrompt(content, options)
}

// getSystemPrompt returns the system prompt for the LLM.
func (c *BedrockClient) getSystemPrompt(_ steps.SummaryOptions) string {
	return summarySystemPrompt
}

// parseSummaryResponse parses the LLM response into a structured Summary.
// Uses the same parser as OpenAI and Ollama clients for consistency.
func (c *BedrockClient) parseSummaryResponse(content string) (*steps.Summary, error) {
	return parseSummaryResponse(content)
}
