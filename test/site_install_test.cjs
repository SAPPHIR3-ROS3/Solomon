const assert = require('node:assert/strict');
const { readFileSync } = require('node:fs');
const { join } = require('node:path');
const { test } = require('node:test');
const { runInNewContext } = require('node:vm');

const source = readFileSync(join(__dirname, '../site/main.js'), 'utf8');
const html = readFileSync(join(__dirname, '../site/index.html'), 'utf8');

function load(navigator) {
  const tabs = [...html.matchAll(/<button[^>]+id="(install-tab-[^"]+)"[^>]+data-cmd="([^"]+)"/g)].map((match) => ({
    id: match[1],
    attributes: { 'data-cmd': match[2] },
    listeners: {},
    setAttribute(key, value) { this.attributes[key] = value; },
    getAttribute(key) { return this.attributes[key]; },
    addEventListener(key, callback) { this.listeners[key] = callback; },
    focus() { this.focused = true; }
  }));
  const target = {};
  const panel = { setAttribute(key, value) { this[key] = value; } };
  const copy = { addEventListener() {} };
  const elements = { '[data-cmd-target]': target, '[data-copy]': copy, "[role='tabpanel']": panel, '[data-copy-status]': {} };
  const install = { querySelectorAll: () => tabs, querySelector: (key) => elements[key] };
  runInNewContext(source, {
    navigator,
    window: { matchMedia: () => ({ matches: true }) },
    document: {
      documentElement: { classList: { add() {} } },
      querySelector: (key) => key === '[data-install]' ? install : null,
      querySelectorAll: () => []
    }
  });
  return { tabs, target, panel };
}

for (const [name, navigator, expected] of [
  ['Windows', { platform: 'Win32' }, 'windows'],
  ['Linux', { platform: 'Linux x86_64' }, 'unix'],
  ['macOS', { platform: 'MacIntel' }, 'unix'],
  ['client hints', { userAgentData: { platform: 'Windows' }, platform: 'Linux' }, 'windows'],
  ['user agent fallback', { userAgent: 'Mozilla/5.0 (Windows NT 10.0; Win64; x64)' }, 'windows'],
  ['unknown platform', {}, 'unix']
]) {
  test('selects the install command for ' + name, () => {
    const { tabs, target, panel } = load(navigator);
    const selected = tabs.find((tab) => tab.id === 'install-tab-' + expected);
    assert.equal(target.textContent, selected.attributes['data-cmd']);
    assert.equal(panel['aria-labelledby'], selected.id);
    for (const tab of tabs) {
      assert.equal(tab.attributes['aria-selected'], String(tab === selected));
      assert.equal(tab.tabIndex, tab === selected ? 0 : -1);
    }
  });
}

test('allows manual selection after automatic detection', () => {
  const { tabs, target, panel } = load({ platform: 'Win32' });
  tabs[0].listeners.click();
  assert.equal(target.textContent, tabs[0].attributes['data-cmd']);
  assert.equal(panel['aria-labelledby'], tabs[0].id);
  tabs[0].listeners.keydown({ key: 'End', preventDefault() {} });
  assert.equal(target.textContent, tabs[2].attributes['data-cmd']);
  assert.equal(tabs[2].focused, true);
});
