import type { Plugin } from "vite";

const DEFAULT_BUDGET = 500_000;
const budgets = [
  { name: "zenuml-runtime", test: /\/node_modules\/@zenuml\/core\/dist\/zenuml\.esm\.mjs$/, maxBytes: 1_800_000 },
  { name: "zenuml-icons", test: /\/node_modules\/@zenuml\/core\/dist\/cloud-icons-[^/]+\.js$/, maxBytes: 1_700_000 },
  { name: "elk-layout", test: /\/node_modules\/elkjs\/lib\/elk\.bundled\.js$/, maxBytes: 1_500_000 },
  { name: "mermaid-parser", test: /\/node_modules\/@mermaid-js\/parser\/dist\/chunks\//, maxBytes: 700_000 },
];

export const largeVendorGroups = budgets.map(({ name, test }) => ({
  name, test, priority: 20, entriesAware: true, includeDependenciesRecursively: false,
}));

export function bundleBudgetsPlugin(): Plugin {
  return {
    name: "solomon-bundle-budgets",
    apply: "build",
    config: () => ({ build: { chunkSizeWarningLimit: Math.max(DEFAULT_BUDGET, ...budgets.map(budget => budget.maxBytes)) / 1000 } }),
    generateBundle(_options, bundle) {
      for (const chunk of Object.values(bundle)) {
        if (chunk.type !== "chunk") continue;
        const modules = Object.keys(chunk.modules).map(id => id.replaceAll("\\", "/"));
        const budget = budgets.find(entry => modules.length > 0 && modules.every(id => entry.test.test(id)));
        const limit = budget?.maxBytes ?? DEFAULT_BUDGET;
        const size = Buffer.byteLength(chunk.code);
        if (size > limit) this.warn(`${chunk.fileName} exceeds its ${limit / 1000} kB bundle budget (${(size / 1000).toFixed(2)} kB).`);
      }
    },
  };
}
