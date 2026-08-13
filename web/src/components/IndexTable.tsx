import { Badge } from './Badge';
import { DetailPopover } from './DetailPopover';
import { Poster } from './Poster';
import { shortDate } from '../lib/format';
import type { Item } from '../lib/types';

interface Props {
  items: Item[];
  admin: boolean;
  desktop: boolean;
  selected: Set<number>;
  onToggle: (id: number) => void;
  onToggleAll: () => void;
  onOpen: (item: Item) => void;
}

export function IndexTable({
  items,
  admin,
  desktop,
  selected,
  onToggle,
  onToggleAll,
  onOpen,
}: Props) {
  return (
    <div className="sg-pad overflow-x-auto py-3">
      <table className="table">
        <thead>
          <tr>
            <th style={{ width: 44 }}>
              <input
                type="checkbox"
                aria-label="Select every row"
                checked={items.length > 0 && selected.size === items.length}
                onChange={onToggleAll}
              />
            </th>
            <th>Title</th>
            <th>Captured</th>
            <th>By</th>
            <th>Source</th>
            <th className="text-right">State</th>
          </tr>
        </thead>
        <tbody>
          {items.map((item) => {
            const title = (
              <button
                type="button"
                className="w-full min-w-0 truncate bg-transparent p-0 text-left text-inherit"
              >
                <b>{item.title}</b> <span className="text-muted">{item.year ?? ''}</span>
              </button>
            );

            return (
              <tr key={item.id}>
                <td>
                  <div className="flex items-center gap-2">
                    <input
                      type="checkbox"
                      aria-label={`Select ${item.title}`}
                      checked={selected.has(item.id)}
                      onChange={() => onToggle(item.id)}
                    />
                    <Poster
                      className="sg-index-poster"
                      path={item.poster_path}
                      title={item.title}
                      size="w185"
                    />
                  </div>
                </td>
                <td className="max-w-[1px]">
                  {desktop ? (
                    <DetailPopover item={item} admin={admin}>
                      {title}
                    </DetailPopover>
                  ) : (
                    <button
                      type="button"
                      className="w-full min-w-0 truncate bg-transparent p-0 text-left text-inherit"
                      onClick={() => onOpen(item)}
                    >
                      <b>{item.title}</b> <span className="text-muted">{item.year ?? ''}</span>
                    </button>
                  )}
                </td>
                <td className="whitespace-nowrap">{shortDate(item.captured_at)}</td>
                <td className="whitespace-nowrap">{item.captured_by?.display_name ?? '—'}</td>
                <td className="whitespace-nowrap">{item.source}</td>
                <td className="text-right">
                  <Badge state={item.status} />
                </td>
              </tr>
            );
          })}
        </tbody>
      </table>
    </div>
  );
}
