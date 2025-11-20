package llm

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/bedrockruntime"
	"github.com/aws/aws-sdk-go-v2/service/bedrockruntime/types"
	"github.com/hashicorp/go-hclog"

	"github.com/hashicorp-forge/hermes/pkg/indexer/pipeline/steps"
)

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
	Region string       // AWS region (default: us-east-1)
	Logger hclog.Logger // Logger (optional)
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
			MaxTokens:   aws.Int32(int32(options.MaxTokens)),
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
	// Truncate content if too long
	maxContentChars := 40000
	if len(content) > maxContentChars {
		content = content[:maxContentChars] + "\n\n[Content truncated...]"
	}

	styleInstruction := ""
	switch options.Style {
	case "executive":
		styleInstruction = "Provide an executive summary suitable for leadership."
	case "technical":
		styleInstruction = "Provide a technical summary with implementation details."
	case "bullet-points":
		styleInstruction = "Focus on concise bullet points of key information."
	default:
		styleInstruction = "Provide a clear and comprehensive summary."
	}

	return fmt.Sprintf(`%s

Please analyze the following document and provide a summary:

%s`, styleInstruction, content)
}

// getSystemPrompt returns the system prompt for the LLM.
func (c *BedrockClient) getSystemPrompt(options steps.SummaryOptions) string {
	return `You are an expert document analyst. Your task is to provide accurate, well-structured summaries of documents.

For each document, provide:
1. EXECUTIVE SUMMARY: A concise 2-3 sentence overview
2. KEY POINTS: The 3-5 most important takeaways (one per line, prefixed with "- ")
3. TOPICS: Main topics covered (comma-separated)
4. TAGS: Relevant tags for categorization (comma-separated)

Format your response as follows:
EXECUTIVE SUMMARY:
[Your executive summary here]

KEY POINTS:
- [First key point]
- [Second key point]
- [Third key point]

TOPICS:
[topic1, topic2, topic3]

TAGS:
[tag1, tag2, tag3]`
}

// parseSummaryResponse parses the LLM response into a structured Summary.
// Uses the same parser as OpenAI and Ollama clients for consistency.
func (c *BedrockClient) parseSummaryResponse(content string) (*steps.Summary, error) {
	summary := &steps.Summary{
		KeyPoints:  []string{},
		Topics:     []string{},
		Tags:       []string{},
		Confidence: 0.8, // Default confidence
	}

	lines := strings.Split(content, "\n")
	currentSection := ""

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		lineUpper := strings.ToUpper(line)

		// Detect section headers (check if line starts with the header)
		if strings.HasPrefix(lineUpper, "EXECUTIVE SUMMARY") {
			currentSection = "executive"
			continue
		}
		if strings.HasPrefix(lineUpper, "KEY POINTS") {
			currentSection = "keypoints"
			continue
		}
		if strings.HasPrefix(lineUpper, "TOPICS") {
			currentSection = "topics"
			continue
		}
		if strings.HasPrefix(lineUpper, "TAGS") {
			currentSection = "tags"
			continue
		}

		// Parse content based on current section
		switch currentSection {
		case "executive":
			if summary.ExecutiveSummary == "" {
				summary.ExecutiveSummary = line
			} else {
				summary.ExecutiveSummary += " " + line
			}

		case "keypoints":
			// Remove bullet prefixes
			point := strings.TrimPrefix(line, "- ")
			point = strings.TrimPrefix(point, "* ")
			point = strings.TrimPrefix(point, "• ")
			if point != line { // Only add if it had a bullet prefix
				summary.KeyPoints = append(summary.KeyPoints, point)
			}

		case "topics":
			// Split by commas
			topics := strings.Split(line, ",")
			for _, topic := range topics {
				topic = strings.TrimSpace(topic)
				if topic != "" {
					summary.Topics = append(summary.Topics, topic)
				}
			}

		case "tags":
			// Split by commas
			tags := strings.Split(line, ",")
			for _, tag := range tags {
				tag = strings.TrimSpace(tag)
				if tag != "" {
					summary.Tags = append(summary.Tags, tag)
				}
			}
		}
	}

	// Validate we got the essential parts
	if summary.ExecutiveSummary == "" {
		return nil, fmt.Errorf("failed to extract executive summary from response")
	}

	return summary, nil
}
