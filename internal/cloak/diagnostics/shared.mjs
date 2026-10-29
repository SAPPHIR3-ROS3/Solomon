export function rejectPaths(args = {}) {
  for (const key of Object.keys(args)) {
    if (/path/i.test(key) && args[key] !== undefined) {
      throw new Error('File paths are unsupported; diagnostic results are returned inline');
    }
  }
}

export function integer(value, fallback, min = 0, max = 100) {
  if (value === undefined) return fallback;
  if (!Number.isSafeInteger(value) || value < min || value > max) {
    throw new Error('Expected an integer between ' + min + ' and ' + max);
  }
  return value;
}

export function text(value, max = 8192) {
  return String(value ?? '').slice(0, max);
}

export function pagination(items, args = {}, fallback = 50, maximum = 100) {
  const pageSize = integer(args.pageSize, fallback, 1, maximum);
  const pageIdx = integer(args.pageIdx, 0, 0, 1000000);
  const start = pageIdx * pageSize;
  return { items: items.slice(start, start + pageSize), total: items.length, pageIdx, pageSize, hasMore: start + pageSize < items.length };
}

export async function detached(session) {
  if (session) await session.detach().catch(() => undefined);
}
