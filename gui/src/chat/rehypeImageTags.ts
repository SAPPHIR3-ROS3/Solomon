import type { Element, Root, RootContent, Text } from "hast";

const excludedElements = new Set(["a", "code", "pre", "script", "style", "textarea"]);

// Decorate parsed text only: Markdown links and code retain their original meaning.
export function rehypeImageTags() {
  return (tree: Root) => {
    function visit(parent: Root | Element) {
      parent.children = parent.children.flatMap((child): RootContent[] => {
        if (child.type === "element") {
          if (!excludedElements.has(child.tagName)) visit(child);
          return [child];
        }
        if (child.type !== "text") return [child];
        return decorateText(child);
      }) as typeof parent.children;
    }
    visit(tree);
  };
}

function decorateText(node: Text): Array<Text | Element> {
  const result: Array<Text | Element> = [];
  let offset = 0;
  const pattern = /\[(?:img-\d+|terminal-L\d+-L\d+)\]/g;
  for (const match of node.value.matchAll(pattern)) {
    if (match.index > offset) {
      result.push({ type: "text", value: node.value.slice(offset, match.index) });
    }
    const isTerminal = match[0].startsWith("[terminal-");
    result.push({
      type: "element",
      tagName: "span",
      properties: { className: isTerminal ? ["composer-image-tag", "chat-terminal-tag"] : ["composer-image-tag", "chat-image-tag"] },
      children: [{ type: "text", value: match[0] }],
    });
    offset = match.index + match[0].length;
  }
  if (!result.length) return [node];
  if (offset < node.value.length) {
    result.push({ type: "text", value: node.value.slice(offset) });
  }
  return result;
}
