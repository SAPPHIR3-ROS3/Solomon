import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import { test } from "node:test";

const source = readFileSync(new URL("../src/App.tsx", import.meta.url), "utf8");
const callback = source.match(/const closeNavigationPanels = useCallback\(\(\) => \{([\s\S]*?)\}, \[panelOverlayOpen\]\);/);
assert.ok(callback);

for (const [name, overlay, mobile, expected] of [
  ["narrow desktop overlay", true, false, [false, false]],
  ["mobile navigation", false, true, [false, false]],
  ["wide desktop docked panels", false, false, []],
]) {
  test(`navigation handles ${name}`, () => {
    const updates = [];
    const close = new Function("panelOverlayOpen", "window", "setIsSidePanelOpen", "setIsRightSidePanelOpen", callback[1]);
    close(overlay, { matchMedia: () => ({ matches: mobile }) }, value => updates.push(value), value => updates.push(value));
    assert.deepEqual(updates, expected);
  });
}

for (const action of ["goHome", "openNewProjectDialog", "openTemporaryWorkspace", "openSettings", "toggleActiveAgents", "toggleCustomization", "openProjectNewChat", "openProjectTerminal"]) {
  test(`${action} uses the shared navigation panel policy`, () => {
    assert.ok(source.includes(`function ${action}() {\n    closeNavigationPanels();`) || new RegExp(`function ${action}\\([^)]*\\) \\{\\s*closeNavigationPanels\\(\\);`).test(source));
  });
}
