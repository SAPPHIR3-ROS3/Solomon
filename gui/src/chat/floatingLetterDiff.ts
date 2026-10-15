// Preserve glyph identity when incomplete Markdown is reformatted. A common
// prefix alone mistakes removals such as **word -> word for entirely new text.
export function reconcileFloatingText(previous: string, current: string) {
  if (previous === current) return {
    addedOffsets: [] as number[],
    retainedOffset: (offset: number) => offset >= 0 && offset < previous.length ? offset : undefined,
  };
  const before = Array.from(previous);
  const after = Array.from(current);
  const offsets = (characters: string[]) => {
    const positions = [0];
    for (const character of characters) positions.push(positions.at(-1)! + character.length);
    return positions;
  };
  const oldOffsets = offsets(before);
  const newOffsets = offsets(after);
  let prefix = 0;
  while (prefix < before.length && prefix < after.length && before[prefix] === after[prefix]) prefix++;
  let oldEnd = before.length, newEnd = after.length;
  while (oldEnd > prefix && newEnd > prefix && before[oldEnd - 1] === after[newEnd - 1]) { oldEnd--; newEnd--; }
  const retained = new Map<number, number>();
  const addedOffsets: number[] = [];
  const rows = oldEnd - prefix, columns = newEnd - prefix;
  if (rows * columns <= 65536) {
    const width = columns + 1;
    const lengths = new Uint16Array((rows + 1) * width);
    for (let row = rows - 1; row >= 0; row--) {
      for (let column = columns - 1; column >= 0; column--) {
        lengths[row * width + column] = before[prefix + row] === after[prefix + column]
          ? 1 + lengths[(row + 1) * width + column + 1]
          : Math.max(lengths[(row + 1) * width + column], lengths[row * width + column + 1]);
      }
    }
    let row = 0, column = 0;
    while (row < rows || column < columns) {
      if (row < rows && column < columns && before[prefix + row] === after[prefix + column]) {
        retained.set(oldOffsets[prefix + row++], newOffsets[prefix + column++]);
      } else if (row < rows && (column === columns || lengths[(row + 1) * width + column] >= lengths[row * width + column + 1])) {
        row++;
      } else {
        addedOffsets.push(newOffsets[prefix + column++]);
      }
    }
  } else {
    // Keep pathological rewrites linear rather than running a quadratic diff on
    // a long response. Normal token appends use only the prefix/suffix paths.
    const positions = new Map<string, number[]>();
    for (let index = prefix; index < oldEnd; index++) {
      const entries = positions.get(before[index]) ?? [];
      entries.push(index);
      positions.set(before[index], entries);
    }
    const cursors = new Map<string, number>();
    let next = prefix;
    for (let index = prefix; index < newEnd; index++) {
      const entries = positions.get(after[index]) ?? [];
      let cursor = cursors.get(after[index]) ?? 0;
      while (cursor < entries.length && entries[cursor] < next) cursor++;
      cursors.set(after[index], cursor);
      if (cursor === entries.length) addedOffsets.push(newOffsets[index]);
      else {
        next = entries[cursor] + 1;
        retained.set(oldOffsets[entries[cursor]], newOffsets[index]);
        cursors.set(after[index], cursor + 1);
      }
    }
  }
  const prefixLength = oldOffsets[prefix];
  const suffixStart = oldOffsets[oldEnd];
  const suffixShift = current.length - previous.length;
  return {
    addedOffsets,
    retainedOffset(offset: number) {
      if (offset < prefixLength) return offset;
      if (offset >= suffixStart && offset < previous.length) return offset + suffixShift;
      return retained.get(offset);
    },
  };
}
