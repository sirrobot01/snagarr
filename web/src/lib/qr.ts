/* A QR encoder small enough to inline. Byte mode, error correction level L,
   versions 1 to 20 — enough for any signed download link. A CDN is blocked and
   a package for this would cost more than the 200 lines below.

   The algorithm is the one in ISO/IEC 18004: encode, add Reed-Solomon parity,
   interleave the blocks, lay the bits out in a zigzag around the function
   patterns, then pick the mask that scores the fewest penalties. */

const ECC_PER_BLOCK = [0, 7, 10, 15, 20, 26, 18, 20, 24, 30, 18, 20, 24, 26, 30, 22, 24, 28, 30, 28, 28];
const BLOCKS = [0, 1, 1, 1, 1, 1, 2, 2, 2, 2, 4, 4, 4, 4, 4, 6, 6, 6, 6, 7, 8];
const MAX_VERSION = 20;

/** Number of data-and-parity modules, which is where every capacity follows from. */
function rawModules(version: number): number {
  let result = (16 * version + 128) * version + 64;
  if (version >= 2) {
    const align = Math.floor(version / 7) + 2;
    result -= (25 * align - 10) * align - 55;
    if (version >= 7) result -= 36;
  }
  return result;
}

function rawCodewords(version: number): number {
  return Math.floor(rawModules(version) / 8);
}

function dataCodewords(version: number): number {
  return rawCodewords(version) - ECC_PER_BLOCK[version] * BLOCKS[version];
}

/** Byte mode carries the length in 8 bits up to version 9, then in 16. */
function countBits(version: number): number {
  return version <= 9 ? 8 : 16;
}

function mul(x: number, y: number): number {
  let z = 0;
  for (let i = 7; i >= 0; i -= 1) {
    z = (z << 1) ^ ((z >>> 7) * 0x11d);
    z ^= ((y >>> i) & 1) * x;
  }
  return z & 0xff;
}

/** The generator polynomial of the Reed-Solomon code, minus its leading term. */
export function rsDivisor(degree: number): number[] {
  const result = new Array<number>(degree).fill(0);
  result[degree - 1] = 1;
  let root = 1;
  for (let i = 0; i < degree; i += 1) {
    for (let j = 0; j < degree; j += 1) {
      result[j] = mul(result[j], root);
      if (j + 1 < degree) result[j] ^= result[j + 1];
    }
    root = mul(root, 2);
  }
  return result;
}

export function rsRemainder(data: readonly number[], divisor: readonly number[]): number[] {
  const result = new Array<number>(divisor.length).fill(0);
  for (const byte of data) {
    const factor = byte ^ (result.shift() as number);
    result.push(0);
    for (let i = 0; i < divisor.length; i += 1) result[i] ^= mul(divisor[i], factor);
  }
  return result;
}

/** The 15 format bits: two bits of error correction level, three of mask, ten
    of BCH parity, all masked with 0x5412 so an all-light code is never valid. */
export function formatBits(mask: number): number {
  const data = (1 << 3) | mask; // level L is 01
  let rem = data;
  for (let i = 0; i < 10; i += 1) rem = (rem << 1) ^ ((rem >>> 9) * 0x537);
  return ((data << 10) | rem) ^ 0x5412;
}

export function versionBits(version: number): number {
  let rem = version;
  for (let i = 0; i < 12; i += 1) rem = (rem << 1) ^ ((rem >>> 11) * 0x1f25);
  return (version << 12) | rem;
}

function bit(value: number, index: number): boolean {
  return ((value >>> index) & 1) !== 0;
}

function alignPositions(version: number): number[] {
  if (version === 1) return [];
  const count = Math.floor(version / 7) + 2;
  const step = Math.ceil((version * 4 + 4) / (count * 2 - 2)) * 2;
  const result = [6];
  for (let pos = version * 4 + 10; result.length < count; pos -= step) result.splice(1, 0, pos);
  return result;
}

/** Mode indicator, length, payload, terminator and pad bytes, as codewords. */
function encodeData(bytes: number[], version: number): number[] {
  const capacity = dataCodewords(version) * 8;
  const bits: boolean[] = [];
  const append = (value: number, width: number) => {
    for (let i = width - 1; i >= 0; i -= 1) bits.push(bit(value, i));
  };

  append(4, 4); // byte mode
  append(bytes.length, countBits(version));
  for (const byte of bytes) append(byte, 8);
  append(0, Math.min(4, capacity - bits.length));
  append(0, (8 - (bits.length % 8)) % 8);
  for (let pad = 0xec; bits.length < capacity; pad ^= 0xec ^ 0x11) append(pad, 8);

  const words: number[] = [];
  for (let i = 0; i < bits.length; i += 8) {
    let word = 0;
    for (let j = 0; j < 8; j += 1) word = (word << 1) | (bits[i + j] ? 1 : 0);
    words.push(word);
  }
  return words;
}

/** Splits the data into blocks, adds parity to each, then reads them back a
    column at a time so a burst of damage never lands inside one block. */
function addEccAndInterleave(data: number[], version: number): number[] {
  const blockCount = BLOCKS[version];
  const eccLen = ECC_PER_BLOCK[version];
  const total = rawCodewords(version);
  const shortCount = blockCount - (total % blockCount);
  const shortLen = Math.floor(total / blockCount);
  const divisor = rsDivisor(eccLen);

  const blocks: number[][] = [];
  for (let i = 0, at = 0; i < blockCount; i += 1) {
    const len = shortLen - eccLen + (i < shortCount ? 0 : 1);
    const chunk = data.slice(at, at + len);
    at += len;
    const ecc = rsRemainder(chunk, divisor);
    if (i < shortCount) chunk.push(0); // padding, skipped on the way out
    blocks.push(chunk.concat(ecc));
  }

  const result: number[] = [];
  for (let i = 0; i < blocks[0].length; i += 1) {
    for (let j = 0; j < blocks.length; j += 1) {
      if (i !== shortLen - eccLen || j >= shortCount) result.push(blocks[j][i]);
    }
  }
  return result;
}

function maskAt(mask: number, x: number, y: number): boolean {
  switch (mask) {
    case 0: return (x + y) % 2 === 0;
    case 1: return y % 2 === 0;
    case 2: return x % 3 === 0;
    case 3: return (x + y) % 3 === 0;
    case 4: return (Math.floor(x / 3) + Math.floor(y / 2)) % 2 === 0;
    case 5: return ((x * y) % 2) + ((x * y) % 3) === 0;
    case 6: return (((x * y) % 2) + ((x * y) % 3)) % 2 === 0;
    default: return (((x + y) % 2) + ((x * y) % 3)) % 2 === 0;
  }
}

const FINDER = [true, false, true, true, true, false, true];

/** Rules 1 to 4 of the spec, scoring how hard a masked matrix is to scan. */
function penalty(modules: boolean[][], size: number): number {
  let score = 0;
  const lines: boolean[][] = [];
  for (let i = 0; i < size; i += 1) {
    lines.push(modules[i]);
    lines.push(modules.map((row) => row[i]));
  }

  for (const line of lines) {
    let run = 1;
    for (let i = 1; i <= size; i += 1) {
      if (i < size && line[i] === line[i - 1]) {
        run += 1;
        continue;
      }
      if (run >= 5) score += 3 + (run - 5);
      run = 1;
    }
    // Rule 3: a finder-like 1:1:3:1:1 run with four light modules beside it.
    for (let i = 0; i + 7 <= size; i += 1) {
      if (!FINDER.every((want, k) => line[i + k] === want)) continue;
      const before = line.slice(Math.max(0, i - 4), i);
      const after = line.slice(i + 7, i + 11);
      const clear = (part: boolean[]) => part.length === 4 && part.every((m) => !m);
      if (clear(before) || clear(after)) score += 40;
    }
  }

  for (let y = 0; y < size - 1; y += 1) {
    for (let x = 0; x < size - 1; x += 1) {
      const c = modules[y][x];
      if (c === modules[y][x + 1] && c === modules[y + 1][x] && c === modules[y + 1][x + 1]) {
        score += 3;
      }
    }
  }

  let dark = 0;
  for (const row of modules) for (const cell of row) if (cell) dark += 1;
  const total = size * size;
  score += (Math.ceil(Math.abs(dark * 20 - total * 10) / total) - 1) * 10;
  return score;
}

export interface QrMatrix {
  size: number;
  modules: boolean[][];
}

/** Encodes text as a QR matrix, or returns null when it does not fit. */
export function encodeQr(text: string): QrMatrix | null {
  const bytes = Array.from(new TextEncoder().encode(text));

  let version = 0;
  for (let v = 1; v <= MAX_VERSION; v += 1) {
    if (4 + countBits(v) + bytes.length * 8 <= dataCodewords(v) * 8) {
      version = v;
      break;
    }
  }
  if (version === 0) return null;

  const size = version * 4 + 17;
  const modules: boolean[][] = Array.from({ length: size }, () => new Array<boolean>(size).fill(false));
  const fixed: boolean[][] = Array.from({ length: size }, () => new Array<boolean>(size).fill(false));

  const set = (x: number, y: number, dark: boolean) => {
    if (x < 0 || y < 0 || x >= size || y >= size) return;
    modules[y][x] = dark;
    fixed[y][x] = true;
  };

  for (let i = 0; i < size; i += 1) {
    set(6, i, i % 2 === 0);
    set(i, 6, i % 2 === 0);
  }
  for (const [cx, cy] of [[3, 3], [size - 4, 3], [3, size - 4]]) {
    for (let dy = -4; dy <= 4; dy += 1) {
      for (let dx = -4; dx <= 4; dx += 1) {
        const dist = Math.max(Math.abs(dx), Math.abs(dy));
        set(cx + dx, cy + dy, dist !== 2 && dist !== 4);
      }
    }
  }
  const aligns = alignPositions(version);
  for (const ax of aligns) {
    for (const ay of aligns) {
      const corner = (ax === 6 && ay === 6) || (ax === 6 && ay === size - 7) || (ax === size - 7 && ay === 6);
      if (corner) continue;
      for (let dy = -2; dy <= 2; dy += 1) {
        for (let dx = -2; dx <= 2; dx += 1) {
          set(ax + dx, ay + dy, Math.max(Math.abs(dx), Math.abs(dy)) !== 1);
        }
      }
    }
  }
  if (version >= 7) {
    const bits = versionBits(version);
    for (let i = 0; i < 18; i += 1) {
      const dark = bit(bits, i);
      set(size - 11 + (i % 3), Math.floor(i / 3), dark);
      set(Math.floor(i / 3), size - 11 + (i % 3), dark);
    }
  }
  // Reserve exactly the format-bit cells, which are written once the mask is
  // chosen. The list skips the timing row and column on purpose.
  for (const [x, y] of formatCells(size)) set(x, y, false);
  set(8, size - 8, true); // the module that is always dark

  const codewords = addEccAndInterleave(encodeData(bytes, version), version);
  let at = 0;
  for (let right = size - 1; right >= 1; right -= 2) {
    if (right === 6) right = 5;
    for (let vert = 0; vert < size; vert += 1) {
      for (let j = 0; j < 2; j += 1) {
        const x = right - j;
        const y = ((right + 1) & 2) === 0 ? size - 1 - vert : vert;
        if (!fixed[y][x] && at < codewords.length * 8) {
          modules[y][x] = bit(codewords[at >>> 3], 7 - (at & 7));
          at += 1;
        }
      }
    }
  }

  let best = { mask: 0, score: Infinity, modules };
  for (let mask = 0; mask < 8; mask += 1) {
    const candidate = modules.map((row, y) =>
      row.map((cell, x) => (fixed[y][x] ? cell : cell !== maskAt(mask, x, y))),
    );
    writeFormat(candidate, size, mask);
    const score = penalty(candidate, size);
    if (score < best.score) best = { mask, score, modules: candidate };
  }
  return { size, modules: best.modules };
}

/** Where the two copies of the 15 format bits live, as [x, y] pairs. */
function formatCells(size: number): [number, number][] {
  const cells: [number, number][] = [];
  for (let i = 0; i <= 5; i += 1) cells.push([8, i]);
  cells.push([8, 7], [8, 8], [7, 8]);
  for (let i = 9; i < 15; i += 1) cells.push([14 - i, 8]);
  for (let i = 0; i < 8; i += 1) cells.push([size - 1 - i, 8]);
  for (let i = 8; i < 15; i += 1) cells.push([8, size - 15 + i]);
  return cells;
}

function writeFormat(modules: boolean[][], size: number, mask: number) {
  const bits = formatBits(mask);
  const cells = formatCells(size);
  for (let i = 0; i < 15; i += 1) {
    const [x, y] = cells[i];
    modules[y][x] = bit(bits, i);
  }
  for (let i = 15; i < 30; i += 1) {
    const [x, y] = cells[i];
    modules[y][x] = bit(bits, i - 15);
  }
}

/** An SVG path covering every dark module, for a one-node render. */
export function qrPath({ size, modules }: QrMatrix): string {
  const parts: string[] = [];
  for (let y = 0; y < size; y += 1) {
    for (let x = 0; x < size; x += 1) {
      if (modules[y][x]) parts.push(`M${x} ${y}h1v1h-1z`);
    }
  }
  return parts.join('');
}
