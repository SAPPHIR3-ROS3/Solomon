import assert from 'node:assert/strict';
import test from 'node:test';
import * as interaction from '../../internal/cloak/interaction.mjs';

function fixture() {
  const records = [];
  const page = {
    evaluateHandle: async () => {
      const uid = 'uid-' + records.length;
      const result = { url: 'http://fixture.test', elements: [{ uid }] };
      const handle = {
        disposed: 0,
        evaluate: async fn => fn({ result }),
        dispose: async () => { handle.disposed++; }
      };
      records.push(handle);
      return handle;
    },
    evaluate: async () => true,
    locator: selector => ({ selector })
  };
  return { page, records };
}

test('only the latest snapshot survives and handles are disposed exactly once', async () => {
  const { page, records } = fixture();
  const tabID = 'snapshot-lifetime';
  await interaction.snapshot(page, tabID, {});
  assert.match((await interaction.resolve(page, tabID, 'uid-0')).selector, /uid-0/);
  await interaction.snapshot(page, tabID, {});
  assert.equal(records[0].disposed, 1);
  await assert.rejects(interaction.resolve(page, tabID, 'uid-0'), /stale element uid/);
  assert.match((await interaction.resolve(page, tabID, 'uid-1')).selector, /uid-1/);
  await interaction.clear(tabID);
  await interaction.clear(tabID);
  assert.deepEqual(records.map(handle => handle.disposed), [1, 1]);
  await assert.rejects(interaction.resolve(page, tabID, 'uid-1'), /stale element uid/);
});

test('destroyed document contexts release snapshot handles', async () => {
  const { page, records } = fixture();
  const tabID = 'snapshot-document';
  await interaction.snapshot(page, tabID, {});
  page.evaluate = async () => { throw new Error('Execution context was destroyed'); };
  await assert.rejects(interaction.resolve(page, tabID, 'uid-0'), /stale element uid/);
  assert.equal(records[0].disposed, 1);
  await interaction.clear(tabID);
  assert.equal(records[0].disposed, 1);
});

test('snapshot serialization failure disposes its unpublished handle', async () => {
  const { page, records } = fixture();
  const capture = page.evaluateHandle;
  page.evaluateHandle = async () => {
    const handle = await capture();
    handle.evaluate = async () => { throw new Error('snapshot interrupted'); };
    return handle;
  };
  await assert.rejects(interaction.snapshot(page, 'snapshot-failure', {}), /snapshot interrupted/);
  assert.equal(records[0].disposed, 1);
  await assert.rejects(interaction.resolve(page, 'snapshot-failure', 'uid-0'), /stale element uid/);
});

test('failed actions dispose their pinned element handles without retrying', async () => {
  const { page } = fixture();
  const tabID = 'snapshot-action';
  await interaction.snapshot(page, tabID, {});
  let disposed = 0;
  let clicks = 0;
  const target = {
    click: async () => { clicks++; throw new Error('Element is not attached'); }
  };
  page.evaluateHandle = async () => ({ asElement: () => target, dispose: async () => { disposed++; } });
  try {
    await assert.rejects(interaction.dispatch('click', { uid: 'uid-0' }, { page, tabID }), /not attached/);
    assert.equal(clicks, 1);
    assert.equal(disposed, 1);
  } finally {
    await interaction.clear(tabID);
  }
});
