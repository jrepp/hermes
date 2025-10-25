"""Command-line interface for Hermes."""

from __future__ import annotations

import json
import sys
from pathlib import Path
from typing import Any

import click
from rich.console import Console
from rich.table import Table

from hc_hermes import Hermes
from hc_hermes.config import HermesConfig
from hc_hermes.exceptions import HermesError
from hc_hermes.utils import create_document_template, parse_markdown_document

console = Console()


def load_config() -> HermesConfig:
    """Load configuration from file or environment."""
    config_path = Path.home() / ".hermes" / "config.yaml"
    if config_path.exists():
        try:
            return HermesConfig.from_file(config_path)
        except Exception as e:
            console.print(f"[yellow]Warning: Failed to load config: {e}[/yellow]")

    return HermesConfig()


def get_client(ctx: click.Context) -> Hermes:
    """Get Hermes client from context."""
    config = ctx.obj.get("config")
    if not config:
        config = load_config()
        ctx.obj["config"] = config

    return Hermes(config=config)


@click.group()
@click.option("--base-url", envvar="HERMES_BASE_URL", help="Hermes server URL")
@click.option("--token", envvar="HERMES_AUTH_TOKEN", help="OAuth bearer token")
@click.option("--debug", is_flag=True, help="Enable debug logging")
@click.pass_context
def cli(ctx: click.Context, base_url: str | None, token: str | None, debug: bool) -> None:
    """Hermes CLI - Interact with HashiCorp Hermes document management system."""
    ctx.ensure_object(dict)

    # Build config
    config_kwargs: dict[str, Any] = {}
    if base_url:
        config_kwargs["base_url"] = base_url
    if token:
        config_kwargs["auth_token"] = token
    if debug:
        config_kwargs["log_level"] = "DEBUG"

    if config_kwargs:
        ctx.obj["config"] = HermesConfig(**config_kwargs)


@cli.group()
def documents() -> None:
    """Document operations."""


@documents.command("get")
@click.argument("doc_id")
@click.option("--json-output", is_flag=True, help="Output as JSON")
@click.pass_context
def documents_get(ctx: click.Context, doc_id: str, json_output: bool) -> None:
    """Get document by ID."""
    try:
        client = get_client(ctx)
        doc = client.documents.get(doc_id)

        if json_output:
            console.print_json(doc.model_dump_json())
        else:
            console.print(f"[bold]{doc.title}[/bold]")
            console.print(f"Document: {doc.full_doc_number or doc_id}")
            console.print(f"Status: {doc.status.value}")
            console.print(f"Product: {doc.product.name if doc.product else 'N/A'}")
            if doc.summary:
                console.print(f"\n{doc.summary}")

    except HermesError as e:
        console.print(f"[red]Error: {e}[/red]", file=sys.stderr)
        sys.exit(1)


@documents.command("get-content")
@click.argument("doc_id")
@click.option("--output", "-o", type=click.Path(), help="Save to file")
@click.pass_context
def documents_get_content(ctx: click.Context, doc_id: str, output: str | None) -> None:
    """Get document content (Markdown)."""
    try:
        client = get_client(ctx)
        content = client.documents.get_content(doc_id)

        if output:
            Path(output).write_text(content.content, encoding="utf-8")
            console.print(f"[green]Content saved to {output}[/green]")
        else:
            console.print(content.content)

    except HermesError as e:
        console.print(f"[red]Error: {e}[/red]", file=sys.stderr)
        sys.exit(1)


@documents.command("update")
@click.argument("doc_id")
@click.option("--title", help="Update title")
@click.option("--status", type=click.Choice(["WIP", "In-Review", "Approved", "Obsolete"]))
@click.option("--summary", help="Update summary")
@click.pass_context
def documents_update(
    ctx: click.Context,
    doc_id: str,
    title: str | None,
    status: str | None,
    summary: str | None,
) -> None:
    """Update document metadata."""
    try:
        client = get_client(ctx)

        updates: dict[str, Any] = {}
        if title:
            updates["title"] = title
        if status:
            updates["status"] = status
        if summary:
            updates["summary"] = summary

        if not updates:
            console.print("[yellow]No updates specified[/yellow]")
            return

        doc = client.documents.update(doc_id, **updates)
        console.print(f"[green]Updated document {doc.full_doc_number or doc_id}[/green]")

    except HermesError as e:
        console.print(f"[red]Error: {e}[/red]", file=sys.stderr)
        sys.exit(1)


@documents.command("update-content")
@click.argument("doc_id")
@click.option("--file", "-f", type=click.Path(exists=True), required=True, help="Markdown file")
@click.pass_context
def documents_update_content(ctx: click.Context, doc_id: str, file: str) -> None:
    """Update document content from file."""
    try:
        client = get_client(ctx)
        content = Path(file).read_text(encoding="utf-8")
        client.documents.update_content(doc_id, content)
        console.print(f"[green]Updated content for {doc_id}[/green]")

    except HermesError as e:
        console.print(f"[red]Error: {e}[/red]", file=sys.stderr)
        sys.exit(1)


@documents.command("create-from-file")
@click.argument("file", type=click.Path(exists=True))
@click.pass_context
def documents_create_from_file(ctx: click.Context, file: str) -> None:
    """Create document from Markdown file with frontmatter."""
    try:
        # Parse document
        parsed = parse_markdown_document(file)

        if not parsed.doc_type or not parsed.product:
            console.print(
                "[red]Error: File must have 'docType' and 'product' in frontmatter[/red]"
            )
            sys.exit(1)

        console.print("[yellow]Note: Document creation requires additional implementation[/yellow]")
        console.print(f"Parsed: {parsed.title} ({parsed.doc_type} - {parsed.product})")

    except Exception as e:
        console.print(f"[red]Error: {e}[/red]", file=sys.stderr)
        sys.exit(1)


@cli.group()
def search() -> None:
    """Search operations."""


@search.command("query")
@click.argument("query")
@click.option("--product", help="Filter by product")
@click.option("--status", type=click.Choice(["WIP", "In-Review", "Approved", "Obsolete"]))
@click.option("--limit", default=20, help="Number of results")
@click.option("--json-output", is_flag=True, help="Output as JSON")
@click.pass_context
def search_query(
    ctx: click.Context,
    query: str,
    product: str | None,
    status: str | None,
    limit: int,
    json_output: bool,
) -> None:
    """Search documents."""
    try:
        client = get_client(ctx)

        filters: dict[str, Any] = {}
        if product:
            filters["product"] = product
        if status:
            filters["status"] = status

        results = client.search.query(query, filters=filters, hits_per_page=limit)

        if json_output:
            console.print_json(results.model_dump_json())
        else:
            table = Table(title=f"Search Results ({results.nb_hits} total)")
            table.add_column("Doc #")
            table.add_column("Title")
            table.add_column("Product")
            table.add_column("Status")

            for hit in results.hits:
                table.add_row(
                    hit.doc_number or "-",
                    hit.title,
                    hit.product or "-",
                    hit.status.value if hit.status else "-",
                )

            console.print(table)

    except HermesError as e:
        console.print(f"[red]Error: {e}[/red]", file=sys.stderr)
        sys.exit(1)


@cli.group()
def projects() -> None:
    """Project operations."""


@projects.command("list")
@click.option("--json-output", is_flag=True, help="Output as JSON")
@click.pass_context
def projects_list(ctx: click.Context, json_output: bool) -> None:
    """List all projects."""
    try:
        client = get_client(ctx)
        projects_list_result = client.projects.list()

        if json_output:
            console.print_json(json.dumps([p.model_dump() for p in projects_list_result]))
        else:
            table = Table(title="Projects")
            table.add_column("Name")
            table.add_column("Title")
            table.add_column("Jira")

            for project in projects_list_result:
                table.add_row(
                    project.name,
                    project.title or "-",
                    "✓" if project.jira_enabled else "✗",
                )

            console.print(table)

    except HermesError as e:
        console.print(f"[red]Error: {e}[/red]", file=sys.stderr)
        sys.exit(1)


@cli.group()
def template() -> None:
    """Document template operations."""


@template.command("create")
@click.argument("output", type=click.Path())
@click.option("--type", "-t", "doc_type", required=True, help="Document type (RFC, PRD, etc.)")
@click.option("--title", required=True, help="Document title")
@click.option("--product", help="Product abbreviation")
@click.option("--author", help="Author email")
@click.option("--summary", help="Document summary")
def template_create(
    output: str,
    doc_type: str,
    title: str,
    product: str | None,
    author: str | None,
    summary: str | None,
) -> None:
    """Create document template."""
    try:
        template_content = create_document_template(
            doc_type=doc_type,
            title=title,
            product=product,
            author=author,
            summary=summary,
        )

        Path(output).write_text(template_content, encoding="utf-8")
        console.print(f"[green]Template created: {output}[/green]")

    except Exception as e:
        console.print(f"[red]Error: {e}[/red]", file=sys.stderr)
        sys.exit(1)


@cli.command("config")
@click.option("--show", is_flag=True, help="Show current configuration")
def config_cmd(show: bool) -> None:
    """Manage configuration."""
    if show:
        config = load_config()
        console.print("[bold]Current Configuration:[/bold]")
        console.print(f"Base URL: {config.base_url}")
        console.print(f"API Version: {config.api_version}")
        console.print(f"Timeout: {config.timeout}s")
        console.print(f"Auth Token: {'Set' if config.auth_token else 'Not set'}")


def main() -> None:
    """Main entry point."""
    cli(obj={})


if __name__ == "__main__":
    main()
