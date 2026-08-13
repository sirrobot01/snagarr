import { useEffect, useState } from 'react';
import { AlertCircle, Grid2X2, Inbox, List as ListIcon, LoaderCircle, RotateCw } from 'lucide-react';
import { useSearch } from 'wouter';

import { DetailSheet } from '../components/DetailSheet';
import { IndexTable } from '../components/IndexTable';
import { PosterGrid } from '../components/PosterGrid';
import { SelectionBar } from '../components/SelectionBar';
import { useIsDesktop } from '../hooks/useMediaQuery';
import { isAdmin, useArchive, useItems, useMe, useSend, useStatus } from '../lib/queries';
import type { Item, Status } from '../lib/types';

type Chip = 'all' | 'ready' | 'pending' | 'reviewing' | 'archived';

const CHIPS: { key: Chip; label: string }[] = [
  { key: 'all', label: 'All' },
  { key: 'ready', label: 'Ready' },
  { key: 'pending', label: 'Pending' },
  { key: 'reviewing', label: 'Reviewing' },
  { key: 'archived', label: 'Archived' },
];

const READY: Status[] = ['available', 'watched'];
const PENDING: Status[] = ['new', 'monitored', 'requested'];

function matches(item: Item, chip: Chip): boolean {
  if (chip === 'ready') return READY.includes(item.status);
  if (chip === 'pending') return PENDING.includes(item.status);
  if (chip === 'reviewing') return item.status === 'needs_review';
  return true;
}

export default function List() {
  const [chip, setChip] = useState<Chip>('all');
  const [view, setView] = useState<'grid' | 'index'>('grid');
  const [selected, setSelected] = useState<Set<number>>(new Set());
  const [openId, setOpenId] = useState<number | null>(null);

  const desktop = useIsDesktop();
  const me = useMe();
  const admin = isAdmin(me.data);
  const items = useItems(chip === 'archived');
  const send = useSend();
  const archive = useArchive();
  const status = useStatus();

  // /list?item=7 opens that item, which is how a search result reaches the
  // detail it promised instead of dropping the reader on the list.
  const query = useSearch();
  useEffect(() => {
    const asked = Number(new URLSearchParams(query).get('item'));
    if (asked) setOpenId(asked);
  }, [query]);

  const visible = (items.data?.items ?? []).filter((item) => matches(item, chip));
  const open = visible.find((item) => item.id === openId) ?? null;

  // A send target nobody has connected cannot work, so it is offered as
  // unavailable rather than as a button that fails.
  const services = status.data?.services;
  const canSend = (services?.radarr ?? false) || (services?.sonarr ?? false);

  function toggle(id: number) {
    setSelected((prev) => {
      const next = new Set(prev);
      if (!next.delete(id)) next.add(id);
      return next;
    });
  }

  function toggleAll() {
    setSelected((prev) =>
      prev.size === visible.length ? new Set() : new Set(visible.map((item) => item.id)),
    );
  }

  function applyToSelection(run: (item: Item) => void) {
    for (const item of visible) if (selected.has(item.id)) run(item);
    setSelected(new Set());
  }

  return (
    <>
      <div className="sg-list-toolbar sg-region sg-pad flex flex-wrap items-center gap-3 py-[14px]">
        <div className="sg-filter-strip flex items-center gap-2" aria-label="Filter your list">
          {CHIPS.map(({ key, label }) => (
            <button
              key={key}
              type="button"
              className="sg-chip"
              data-on={chip === key ? '1' : undefined}
              aria-pressed={chip === key}
              onClick={() => {
                setChip(key);
                setSelected(new Set());
              }}
            >
              {label}
            </button>
          ))}
        </div>

        <span className="sg-list-count sg-k ml-auto">
          {visible.length} {visible.length === 1 ? 'item' : 'items'}
        </span>

        <div className="sg-view-toggle" role="group" aria-label="List layout">
          <button
            type="button"
            className="sg-view-button"
            data-on={view === 'grid' ? '1' : undefined}
            aria-pressed={view === 'grid'}
            aria-label="Poster grid"
            onClick={() => setView('grid')}
          >
            <Grid2X2 aria-hidden="true" size={16} />
          </button>
          <button
            type="button"
            className="sg-view-button"
            data-on={view === 'index' ? '1' : undefined}
            aria-pressed={view === 'index'}
            aria-label="Table view"
            onClick={() => setView('index')}
          >
            <ListIcon aria-hidden="true" size={17} />
          </button>
        </div>
      </div>

      {items.isPending && (
        <p className="sg-k sg-pad flex items-center gap-2 py-6" role="status">
          <LoaderCircle className="animate-spin" aria-hidden="true" size={14} /> Loading your list…
        </p>
      )}
      {items.isError && (
        <div className="sg-pad flex flex-col items-start gap-3 py-6">
          <p className="sg-k m-0 flex items-center gap-2">
            <AlertCircle aria-hidden="true" size={15} /> We couldn’t load your list
          </p>
          <button type="button" className="btn btn-secondary mt-2" onClick={() => items.refetch()}>
            <RotateCw aria-hidden="true" size={15} />
            Retry
          </button>
        </div>
      )}

      {!items.isPending && !items.isError && visible.length === 0 && (
        <section className="sg-empty sg-pad flex flex-col items-center py-10 text-center">
          <span className="sg-empty-icon" aria-hidden="true">
            <Inbox size={25} />
          </span>
          <h3 className="mt-3 text-[21px]">Nothing in {chip === 'all' ? 'your list' : chip}</h3>
          <p className="text-muted m-0 max-w-[320px] text-[13px]">
            {chip === 'archived'
              ? 'Titles you archive will appear here.'
              : 'Try another filter, or snag a title from the search page.'}
          </p>
        </section>
      )}

      {view === 'grid' ? (
        <PosterGrid
          items={visible}
          admin={admin}
          desktop={desktop}
          openId={openId}
          onOpen={setOpenId}
        />
      ) : (
        <IndexTable
          items={visible}
          admin={admin}
          desktop={desktop}
          selected={selected}
          onToggle={toggle}
          onToggleAll={toggleAll}
          openId={openId}
          onOpen={setOpenId}
        />
      )}

      {selected.size > 0 && (
        <SelectionBar
          count={selected.size}
          admin={admin}
          canSend={canSend}
          onArchive={() => applyToSelection((item) => archive.mutate({ item, value: true }))}
          onSend={() =>
            applyToSelection((item) =>
              send.mutate({ item, value: item.media_type === 'tv' ? 'sonarr' : 'radarr' }),
            )
          }
        />
      )}

      {!desktop && <DetailSheet item={open} admin={admin} onClose={() => setOpenId(null)} />}
    </>
  );
}
