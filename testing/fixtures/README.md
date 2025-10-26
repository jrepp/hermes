# Document Fixtures and Templates

This directory contains reusable document templates for testing.

## Structure

```
fixtures/
├── rfcs/
│   └── RFC-TEMPLATE.md          # RFC template
├── prds/
│   └── PRD-TEMPLATE.md          # PRD template
├── meetings/
│   └── (meeting notes templates)
└── README.md                     # This file
```

## Templates

### RFC Template

**File**: `rfcs/RFC-TEMPLATE.md`

Standard Request for Comments template with:
- YAML frontmatter with Hermes UUID
- Standard RFC sections (Summary, Background, Proposal, etc.)
- Markdown formatting
- Placeholder content

**Usage**:
```bash
# Manually
cp fixtures/rfcs/RFC-TEMPLATE.md workspaces/testing/rfcs/RFC-001-my-rfc.md

# Or use the generator
source scripts/lib/document-generator.sh
generate_rfc 1 "$(generate_uuid)" "My RFC Title" > workspace/testing/rfcs/RFC-001.md
```

### PRD Template

**File**: `prds/PRD-TEMPLATE.md`

Product Requirements Document template with:
- Executive summary
- User stories and personas
- Functional/non-functional requirements
- Timeline and stakeholders
- Risk assessment

**Usage**:
```bash
# Manually
cp fixtures/prds/PRD-TEMPLATE.md workspaces/testing/prds/PRD-001-my-feature.md

# Or use the generator
source scripts/lib/document-generator.sh
generate_prd 1 "$(generate_uuid)" "My Feature" > workspace/testing/prds/PRD-001.md
```

## Customizing Templates

### Add New Template

1. Create template file:
   ```bash
   cat > fixtures/my-type/MY-TEMPLATE.md <<'EOF'
   ---
   hermes-uuid: TEMPLATE-TYPE-001
   document-type: MY_TYPE
   # ... frontmatter
   ---
   
   # Template content
   EOF
   ```

2. Add generator function to `scripts/lib/document-generator.sh`:
   ```bash
   generate_my_type() {
       local number="$1"
       local uuid="${2:-$(generate_uuid)}"
       # ... implementation
   }
   ```

3. Document in this README

### Frontmatter Fields

**Required**:
- `hermes-uuid`: Stable UUID for the document
- `document-type`: RFC, PRD, FRD, MEETING, etc.
- `title`: Human-readable title
- `created`: ISO 8601 timestamp
- `modified`: ISO 8601 timestamp

**Optional**:
- `document-number`: RFC-001, PRD-042, etc.
- `status`: draft, review, approved, published
- `authors`: Array of email addresses
- `tags`: Array of keywords for search
- `stakeholders`: Relevant team members
- Any custom metadata

### Example Frontmatter

```yaml
---
hermes-uuid: 550e8400-e29b-41d4-a716-446655440000
document-type: RFC
document-number: RFC-042
status: review
title: "API Gateway Redesign"
authors:
  - alice@example.com
  - bob@example.com
created: 2025-10-01T10:00:00Z
modified: 2025-10-15T14:30:00Z
tags:
  - api
  - gateway
  - infrastructure
stakeholders:
  - eng-lead@example.com
  - product@example.com
---
```

## Generator Functions

The `scripts/lib/document-generator.sh` library provides programmatic generation:

**Available Generators**:
- `generate_rfc(number, uuid, title, status, author, created)`
- `generate_prd(number, uuid, title, status, author, created)`
- `generate_meeting_notes(number, uuid, title, attendees, date, created)`
- `generate_doc_page(title, uuid, category, author, created)`

**See**: `scripts/README.md` for detailed usage

## Best Practices

### Test Data Safety
- ✅ Use `example.com` for all email addresses
- ✅ Use generic/fictional names
- ✅ Generate random UUIDs (not production IDs)
- ✅ Keep content generic and public-safe
- ✅ No real credentials, internal URLs, or sensitive data

### Naming Conventions
- **Files**: `TYPE-NNN-kebab-case-title.md`
  - Examples: `RFC-001-api-gateway.md`, `PRD-042-search-feature.md`
- **Document Numbers**: `TYPE-NNN` (zero-padded to 3 digits)
  - Examples: `RFC-001`, `PRD-042`, `MEET-005`
- **UUIDs**: Lowercase, standard format
  - Example: `550e8400-e29b-41d4-a716-446655440000`

### Content Guidelines
- Use Markdown formatting (headers, lists, code blocks)
- Include realistic section structure
- Add searchable keywords for testing
- Keep content brief but representative
- Include metadata for filtering/faceting

## Template Variables

When creating templates, use these placeholders:

| Placeholder | Replace With | Example |
|-------------|--------------|---------|
| `XXX` | Document number | `001`, `042` |
| `YYYY-MM-DD` | Date | `2025-10-24` |
| `TEMPLATE-TYPE-001` | Unique UUID | `550e8400-...` |
| `author@example.com` | Author email | `alice@example.com` |
| `Title Goes Here` | Actual title | `API Gateway Design` |
| `vX.Y.Z` | Version number | `v1.2.0` |

## References

- **Generator Library**: `../scripts/lib/document-generator.sh`
- **Seed Script**: `../scripts/seed-workspaces.sh`
- **Architecture**: `../DISTRIBUTED_TESTING_ENHANCEMENTS.md`
- **Projects Config**: `../projects/README.md`
