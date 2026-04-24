// Package llm provides LLM integration for Hermes.
package llm

import (
	"fmt"
	"strings"

	"github.com/hashicorp-forge/hermes/pkg/indexer/pipeline/steps"
)

const (
	defaultMaxContentChars = 40000

	providerOpenAI  = "openai"
	providerBedrock = "bedrock"
	providerOllama  = "ollama"

	summaryStyleExecutive    = "executive"
	summaryStyleTechnical    = "technical"
	summaryStyleBulletPoints = "bullet-points"

	styleInstructionExecutive    = "Provide an executive summary suitable for leadership."
	styleInstructionTechnical    = "Provide a technical summary with implementation details."
	styleInstructionBulletPoints = "Focus on concise bullet points of key information."
	styleInstructionDefault      = "Provide a clear and comprehensive summary."

	sectionExecutive = "executive"
	sectionKeyPoints = "keypoints"
	sectionTopics    = "topics"
	sectionTags      = "tags"

	summarySystemPrompt = `You are an expert document analyst. Your task is to provide accurate, well-structured summaries of documents.

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
)

func buildSummaryPrompt(content string, options steps.SummaryOptions) string {
	if len(content) > defaultMaxContentChars {
		content = content[:defaultMaxContentChars] + "\n\n[Content truncated...]"
	}

	styleInstruction := styleInstructionDefault
	switch options.Style {
	case summaryStyleExecutive:
		styleInstruction = styleInstructionExecutive
	case summaryStyleTechnical:
		styleInstruction = styleInstructionTechnical
	case summaryStyleBulletPoints:
		styleInstruction = styleInstructionBulletPoints
	}

	return fmt.Sprintf(`%s

Please analyze the following document and provide a summary:

%s`, styleInstruction, content)
}

func parseSummaryResponse(content string) (*steps.Summary, error) {
	summary := &steps.Summary{
		KeyPoints:  []string{},
		Topics:     []string{},
		Tags:       []string{},
		Confidence: 0.8,
	}

	lines := strings.Split(content, "\n")
	currentSection := ""

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		lineUpper := strings.ToUpper(line)
		switch {
		case strings.HasPrefix(lineUpper, "EXECUTIVE SUMMARY"):
			currentSection = sectionExecutive
			continue
		case strings.HasPrefix(lineUpper, "KEY POINTS"):
			currentSection = sectionKeyPoints
			continue
		case strings.HasPrefix(lineUpper, "TOPICS"):
			currentSection = sectionTopics
			continue
		case strings.HasPrefix(lineUpper, "TAGS"):
			currentSection = sectionTags
			continue
		}

		switch currentSection {
		case sectionExecutive:
			if summary.ExecutiveSummary == "" {
				summary.ExecutiveSummary = line
			} else {
				summary.ExecutiveSummary += " " + line
			}
		case sectionKeyPoints:
			point := strings.TrimPrefix(line, "- ")
			point = strings.TrimPrefix(point, "* ")
			point = strings.TrimPrefix(point, "• ")
			if point != line {
				summary.KeyPoints = append(summary.KeyPoints, point)
			}
		case sectionTopics:
			appendCSVFields(&summary.Topics, line)
		case sectionTags:
			appendCSVFields(&summary.Tags, line)
		}
	}

	if summary.ExecutiveSummary == "" {
		return nil, fmt.Errorf("failed to extract executive summary from response")
	}

	return summary, nil
}

func appendCSVFields(dst *[]string, line string) {
	fields := strings.Split(line, ",")
	for _, field := range fields {
		field = strings.TrimSpace(field)
		if field != "" {
			*dst = append(*dst, field)
		}
	}
}
