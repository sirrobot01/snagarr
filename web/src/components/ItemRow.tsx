import { ChevronRight } from 'lucide-react';
import { Badge } from './Badge';
import { Poster } from './Poster';
import { shortDate } from '../lib/format';
import type { Item } from '../lib/types';

interface Props {
  item: Item;
  active?: boolean;
  onSelect?: (item: Item) => void;
}

export function ItemRow({ item, active, onSelect }: Props) {
  return (
    <button
      type="button"
      className="sg-row"
      data-active={active ? '1' : undefined}
      onClick={() => onSelect?.(item)}
    >
      <Poster className="sg-row-poster" path={item.poster_path} title={item.title} size="w185" />

      <span className="min-w-0 flex-1">
        <span className="sg-row-title block truncate">{item.title}</span>
        <span className="sg-row-meta block truncate">
          {[item.year, shortDate(item.captured_at), item.captured_by?.username]
            .filter(Boolean)
            .join(' · ')}
        </span>
      </span>

      <Badge state={item.status} />
      <ChevronRight className="sg-row-chevron" aria-hidden="true" size={17} />
    </button>
  );
}
