import type { ChatFileChange } from "./chatTypes";

export type FileChangeNode = {
  name: string;
  path: string;
  added: number;
  removed: number;
  files: number;
  children: FileChangeNode[];
  change?: ChatFileChange;
};

export function fileChangeTree(changes: readonly ChatFileChange[]): FileChangeNode[] {
  const root: FileChangeNode = { name: "", path: "", added: 0, removed: 0, files: 0, children: [] };
  for (const change of changes) {
    const parts = change.path.split("/").filter(Boolean);
    if (!parts.length) continue;
    // External changes have absolute paths. Keep their root both in the
    // displayed folder chain and in its identity, separate from project files.
    if (change.path.startsWith("/")) parts[0] = `/${parts[0]}`;
    let parent = root;
    for (let index = 0; index < parts.length; index++) {
      const path = parts.slice(0, index + 1).join("/");
      let node = parent.children.find(child => child.path === path);
      if (!node) {
        node = { name: parts[index], path, added: 0, removed: 0, files: 0, children: [] };
        parent.children.push(node);
      }
      node.added += change.added;
      node.removed += change.removed;
      node.files++;
      if (index === parts.length - 1) node.change = change;
      parent = node;
    }
  }
  function compact(nodes: FileChangeNode[]): FileChangeNode[] {
    return nodes.sort((a, b) => Number(Boolean(a.change)) - Number(Boolean(b.change)) || a.name.localeCompare(b.name)).map(node => {
      while (!node.change && node.children.length === 1 && !node.children[0].change) {
        const child = node.children[0];
        node.name += `/${child.name}`;
        node.path = child.path;
        node.children = child.children;
      }
      node.children = compact(node.children);
      return node;
    });
  }
  return compact(root.children);
}
