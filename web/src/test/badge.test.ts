import { describe, expect, it } from 'vitest';
import { badgeFor } from '../lib/format';
import type { Status } from '../lib/types';

describe('badgeFor', () => {
  const cases: [Status, string, string][] = [
    ['available', 'In library', 'sg-lib'],
    ['watched', 'In library', 'sg-lib'],
    ['monitored', 'Monitored', 'sg-mon'],
    ['requested', 'Requested', 'sg-req'],
    ['needs_review', 'Needs review', 'sg-rev'],
    ['new', 'New', 'sg-new'],
  ];

  it.each(cases)('maps %s to %s', (state, label, className) => {
    expect(badgeFor(state)).toEqual({ label, className });
  });

  it('falls back to New for a state the API adds later', () => {
    expect(badgeFor('unknown' as Status).label).toBe('New');
  });
});
