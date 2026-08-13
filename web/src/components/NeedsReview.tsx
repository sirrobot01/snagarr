import { useState } from 'react';
import { AlertTriangle, ChevronDown } from 'lucide-react';
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
        <h2 className="sg-review-title flex items-center gap-2">
          <AlertTriangle aria-hidden="true" size={17} /> Needs your review
          <span className="sg-review-count">{items.length}</span>
        </h2>
        <span className="sg-k">We kept the original capture</span>
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
              {[shortDate(item.captured_at), item.source].filter(Boolean).join(' · ')}
            </span>
            <ChevronDown aria-hidden="true" size={16} />
          </button>
        ))}
    </section>
  );
}
