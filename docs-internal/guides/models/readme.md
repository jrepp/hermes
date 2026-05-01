---
id: model-cards-readme
title: "Model Cards"
status: Reference
created: 2026-05-01
author: Hermes Team
project_id: hermes
doc_uuid: eb1789ff-6742-4d3c-bcb7-5d595a093ea8
type: Guide
subtype: Reference
tags: [models, embeddings, ai, reference]
---

# Model Cards

> Reference cards for AI models that Hermes can use through local or provider-backed integrations. These cards record provenance, intended use, operational notes, and known limitations before a model is promoted into pipeline configuration.

## Embedding Models

- [EmbeddingGemma 300M](embeddinggemma-300m.md) — local/open embedding model option for Ollama-compatible embedding pipelines.

## Authoring Notes

- Record the upstream model page, authors, license or terms, intended use, dimensions, context limits, and operational caveats.
- Link any provider-specific setup guide required to run the model locally.
- Keep model cards descriptive. Binding architecture decisions still belong in ADRs.
