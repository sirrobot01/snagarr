import { Badge } from './Badge';
import { DetailPopover } from './DetailPopover';
import { Poster } from './Poster';
import { shortDate } from '../lib/format';
import type { Item } from '../lib/types';

interface Props {
  items: Item[];
  admin: boolean;
  desktop: boolean;
  openId: number | null;
  onOpen: (id: number | null) => void;
}

function Cell({ item }: { item: Item }) {
  return (
    <>
      <Poster fill path={item.poster_path} title={item.title} size="w342" />
      <span className="sg-cell-title line-clamp-2">{item.title}</span>
      <span className="sg-cell-sub">
        {[item.year, shortDate(item.captured_at)].filter(Boolean).join(' · ')}
      </span>
      <Badge state={item.status} />
    </>
  );
}

export function PosterGrid({ items, admin, desktop, openId, onOpen }: Props) {
  return (
    <div className="sg-grid">
      {items.map((item) =>
        desktop ? (
          <DetailPopover
            key={item.id}
            item={item}
            admin={admin}
            open={openId === item.id}
            onOpenChange={(next) => onOpen(next ? item.id : null)}
          >
            <button type="button" className="sg-cell" aria-label={`Open details for ${item.title}`}>
              <Cell item={item} />
            </button>
          </DetailPopover>
        ) : (
          <button
            key={item.id}
            type="button"
            className="sg-cell"
            aria-label={`Open details for ${item.title}`}
            onClick={() => onOpen(item.id)}
          >
            <Cell item={item} />
          </button>
        ),
      )}
    </div>
  );
}
