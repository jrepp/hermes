import { helper } from "@ember/component/helper";
import MarkdownIt from "markdown-it";

const md = new MarkdownIt({
  html: true,
  linkify: true,
  typographer: true,
});

export function markdownToHtml([markdown]: [string]): string {
  if (!markdown) {
    return "";
  }
  return md.render(markdown);
}

export default helper(markdownToHtml);

declare module "@glint/environment-ember-loose/registry" {
  export default interface Registry {
    "markdown-to-html": typeof markdownToHtml;
  }
}
