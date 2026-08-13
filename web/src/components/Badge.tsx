import { badgeFor } from '../lib/format';
import type { Status } from '../lib/types';

export function Badge({ state }: { state: Status }) {
  const { label, className } = badgeFor(state);
  return (
    <span className={`sg-b ${className}`} title={`Status: ${label.toLowerCase()}`}>
      {label}
    </span>
  );
}
