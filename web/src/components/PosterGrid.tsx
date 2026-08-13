import { Badge } from './Badge';
import { DetailPopover } from './DetailPopover';
import { Poster } from './Poster';
import { shortDate } from '../lib/format';
import type { Item } from '../lib/types';

interface Props {
  items: Item[];
  admin: boolean;
  desktop: boolean;
  onOpen: (item: Item) => void;
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

export function PosterGrid({ items, admin, desktop, onOpen }: Props) {
  return (
    <div className="sg-grid">
      {items.map((item) =>
        desktop ? (
          <DetailPopover key={item.id} item={item} admin={admin}>
            <button type="button" className="sg-cell">
              <Cell item={item} />
            </button>
          </DetailPopover>
        ) : (
          <button key={item.id} type="button" className="sg-cell" onClick={() => onOpen(item)}>
            <Cell item={item} />
          </button>
        ),
      )}
    </div>
  );
}
