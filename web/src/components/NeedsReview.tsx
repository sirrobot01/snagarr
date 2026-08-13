import { useState } from 'react';
import { NeedsReviewCard } from './NeedsReviewCard';
import { shortDate } from '../lib/format';
import type { Item } from '../lib/types';

interface Props {
  items: Item[];
  onSearchManually: (raw: string) => void;
}

export function NeedsReview({ items, onSearchManually }: Props) {
  const [expanded, setExpanded] = useState<number | null>(null);
  if (items.length === 0) return null;

  const open = items.find((item) => item.id === expanded) ?? items[0];

  return (
    <section className="sg-region">
      <div className="sg-review-head">
        <h2 className="sg-review-title">NEEDS REVIEW — {items.length}</h2>
        <span className="sg-k">NOTHING IS EVER DROPPED</span>
      </div>

      <NeedsReviewCard key={open.id} item={open} onSearchManually={onSearchManually} />

      {items
        .filter((item) => item.id !== open.id)
        .map((item) => (
          <button
            key={item.id}
            type="button"
            className="sg-review-collapsed"
            onClick={() => setExpanded(item.id)}
          >
            <span className="min-w-0 truncate">{item.raw_input}</span>
            <span className="sg-k shrink-0">
              {[shortDate(item.captured_at), item.source].filter(Boolean).join(' · ').toUpperCase()}
            </span>
          </button>
        ))}
    </section>
  );
}
