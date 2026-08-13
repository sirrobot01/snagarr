import { describe, expect, it } from 'vitest';
import { encodeQr, formatBits, rsDivisor, rsRemainder, versionBits } from '../lib/qr';

/* The 15-bit format strings for error correction level L, masks 0 to 7,
   copied from ISO/IEC 18004 Annex C. */
const FORMAT_L = [0x77c4, 0x72f3, 0x7daa, 0x789d, 0x662f, 0x6318, 0x6c41, 0x6976];

describe('qr encoder', () => {
  it('produces the published format and version bit strings', () => {
    expect(FORMAT_L.map((_, mask) => formatBits(mask))).toEqual(FORMAT_L);
    expect(versionBits(7)).toBe(0x07c94);
  });

  it('produces parity that divides by the generator polynomial', () => {
    const divisor = rsDivisor(10);
    const data = [32, 91, 11, 120, 209, 114, 220, 77, 67, 64, 236, 17, 236, 17, 236, 17];
    const parity = rsRemainder(data, divisor);
    expect(rsRemainder(data.concat(parity), divisor).every((byte) => byte === 0)).toBe(true);
  });

  it('lays out the function patterns a scanner looks for', () => {
    const qr = encodeQr('https://snagarr.example.com/api/v1/users/1/shortcut?sig=abc123&exp=99');
    expect(qr).not.toBeNull();
    if (!qr) return;

    const { size, modules } = qr;
    expect(size).toBe(4 * 4 + 17); // 68 bytes needs version 4 at level L

    // Three finder patterns: dark ring, light ring, dark 3×3 core.
    for (const [cx, cy] of [[3, 3], [size - 4, 3], [3, size - 4]]) {
      expect(modules[cy][cx]).toBe(true);
      expect(modules[cy - 1][cx - 1]).toBe(true);
      expect(modules[cy - 2][cx]).toBe(false);
      expect(modules[cy - 3][cx]).toBe(true);
    }

    // Timing patterns alternate along row 6 and column 6.
    for (let i = 8; i < size - 8; i += 1) {
      expect(modules[6][i]).toBe(i % 2 === 0);
      expect(modules[i][6]).toBe(i % 2 === 0);
    }

    expect(modules[size - 8][8]).toBe(true); // the module that is always dark
    expect(encodeQr('x'.repeat(1000))).toBeNull();
  });
});
