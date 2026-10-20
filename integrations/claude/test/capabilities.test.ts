import assert from 'node:assert/strict';
import { readFile } from 'node:fs/promises';
import test from 'node:test';
import { createSdkMcpServer, query } from '@anthropic-ai/claude-agent-sdk';

const root = new URL('../node_modules/@anthropic-ai/claude-agent-sdk/', import.meta.url);
const declarations = await readFile(new URL('sdk.d.ts', root), 'utf8');

test('approved SDK version and public entrypoints are available', async () => {
  const manifest = JSON.parse(await readFile(new URL('package.json', root), 'utf8'));
  assert.equal(manifest.version, '0.3.305');
  assert.equal(manifest.claudeCodeVersion, '2.1.305');
  assert.equal(typeof query, 'function');
  assert.equal(typeof createSdkMcpServer, 'function');
});

test('query input documents user envelopes, not arbitrary history', () => {
  assert.match(declarations, /prompt: string \| AsyncIterable<SDKUserMessage>/);
  assert.match(declarations, /type SDKUserMessage = \{\s+type: 'user'/);
  assert.match(declarations, /It will be merged into the next user message that does query/);
});

test('session-store records are documented as opaque internal data', () => {
  assert.match(declarations, /Returned entries must be deep-equal to what was\s+\* appended/);
  assert.match(declarations, /CLI-internal and not part\s+\* of the SDK API surface/);
  assert.match(declarations, /treat entries as pass-through blobs/);
});

test('tool interception and process termination interfaces exist', () => {
  assert.match(declarations, /Disable all built-in tools/);
  assert.match(declarations, /type PreToolUseHookInput/);
  assert.match(declarations, /tool_use_id: string/);
  assert.match(declarations, /permissionDecision\?: HookPermissionDecision/);
  assert.match(declarations, /Close the query and terminate the underlying process/);
});
