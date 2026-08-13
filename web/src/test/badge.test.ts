import { describe, expect, it } from 'vitest';
import { badgeFor } from '../lib/format';
import type { Status } from '../lib/types';

describe('badgeFor', () => {
  const cases: [Status, string, string][] = [
    ['available', 'IN LIBRARY', 'sg-lib'],
    ['watched', 'IN LIBRARY', 'sg-lib'],
    ['monitored', 'MONITORED', 'sg-mon'],
    ['requested', 'REQUESTED', 'sg-req'],
    ['needs_review', 'NEEDS REVIEW', 'sg-rev'],
    ['new', 'NEW', 'sg-new'],
  ];

  it.each(cases)('maps %s to %s', (state, label, className) => {
    expect(badgeFor(state)).toEqual({ label, className });
  });

  it('falls back to NEW for a state the API adds later', () => {
    expect(badgeFor('unknown' as Status).label).toBe('NEW');
  });
});
