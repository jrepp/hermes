---
id: model-card-embeddinggemma-300m
title: "EmbeddingGemma 300M Model Card"
status: Reference
created: 2026-05-01
author: Hermes Team
project_id: hermes
doc_uuid: 19a2b40c-cb7b-4119-b759-14e59e6690fc
type: Guide
subtype: Reference
tags: [models, embeddings, ollama, hugging-face, gemma]
related:
  - RFC-014
---

# Model Card: EmbeddingGemma 300M

> Records provenance and Hermes usage notes for `unsloth/embeddinggemma-300m`, a 300M parameter open embedding model derived from Google's EmbeddingGemma release. Hermes treats it as a local embedding model candidate for Ollama-compatible pipelines, not as a default production model.

## Provenance

- Upstream safetensors model page: [unsloth/embeddinggemma-300m](https://huggingface.co/unsloth/embeddinggemma-300m)
- Ollama-compatible GGUF model page: [unsloth/embeddinggemma-300m-GGUF](https://huggingface.co/unsloth/embeddinggemma-300m-GGUF)
- Model family: EmbeddingGemma
- Authors listed by upstream card: Google DeepMind
- Repository publisher: Unsloth AI on Hugging Face
- Paper: [EmbeddingGemma: Powerful and Lightweight Text Representations](https://arxiv.org/abs/2509.20354)
- License / terms: Hugging Face page lists `gemma`; upstream card links to [Gemma Terms of Use](https://ai.google.dev/gemma/terms)
- Primary tasks: retrieval, semantic similarity, classification, clustering, question answering, fact verification, and code retrieval embeddings

## Technical Notes

- Parameter count: 300M
- Default output dimension: 768
- Optional Matryoshka dimensions listed upstream: 512, 256, 128 after truncation and re-normalization
- Maximum input context listed upstream: 2048 tokens
- Training language coverage listed upstream: 100+ spoken languages
- Precision caveat listed upstream: activations do not support `float16`; use `float32` or `bfloat16` as appropriate for the runtime

## Hermes Usage

Hermes recognizes these model names as local/Ollama embedding candidates:

- `embeddinggemma-300m`
- `unsloth/embeddinggemma-300m`
- `hf.co/unsloth/embeddinggemma-300m-GGUF:Q4_0`

Recommended pipeline step configuration:

```hcl
config = {
  embeddings = {
    provider = "ollama"
    model = "hf.co/unsloth/embeddinggemma-300m-GGUF:Q4_0"
    dimensions = 768
  }
}
```

The safetensors repository (`unsloth/embeddinggemma-300m`) is not directly pullable by Ollama. For Ollama, use the GGUF repository or a local alias created from that artifact:

```bash
ollama pull hf.co/unsloth/embeddinggemma-300m-GGUF:Q4_0
```

If an Ollama runtime exposes the model under a local alias, use that alias in `model` and keep `dimensions = 768` unless the runtime intentionally truncates to a smaller Matryoshka dimension.

## Prompting Notes

The upstream card recommends typed prefixes for retrieval quality:

- Query retrieval: `task: search result | query: {content}`
- Document retrieval: `title: {title | "none"} | text: {content}`
- Question answering: `task: question answering | query: {content}`
- Code retrieval: `task: code retrieval | query: {content}`

Hermes currently passes raw text to the embedding client. Add a dedicated prompt-formatting decision before relying on query/document asymmetric prompts in production ranking.

## Limitations And Risks

- The Hugging Face card is hosted by Unsloth, while the authorship and terms reference Google's EmbeddingGemma release. Keep both provenance facts visible when reviewing license and distribution terms.
- Model quality depends on the local runtime packaging and quantization. Re-validate dimensions and retrieval behavior for the exact Ollama model artifact used.
- The GGUF `Q4_0` artifact has been smoke-tested through Ollama's `/api/embeddings` endpoint locally and returned a 768-dimensional vector.
- Smaller Matryoshka dimensions require truncation and re-normalization support; Hermes currently only validates the returned vector length.
- Local embeddings avoid sending document text to a third-party API, but generated vectors can still encode sensitive content and should be handled as derived document data.

## References

- [Ollama integration guide](../integrations/ollama.md)
- [Indexer cutover trajectory](../../plans/trajectory-002-indexer-cutover.md)
- [Model cards index](readme.md)
