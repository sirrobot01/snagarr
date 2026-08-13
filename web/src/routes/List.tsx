import { useState } from 'react';

import { DetailSheet } from '../components/DetailSheet';
import { IndexTable } from '../components/IndexTable';
import { PosterGrid } from '../components/PosterGrid';
import { SelectionBar } from '../components/SelectionBar';
import { useIsDesktop } from '../hooks/useMediaQuery';
import { isAdmin, useArchive, useItems, useMe, useSend } from '../lib/queries';
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
  const [open, setOpen] = useState<Item | null>(null);

  const desktop = useIsDesktop();
  const me = useMe();
  const admin = isAdmin(me.data);
  const items = useItems(chip === 'archived');
  const send = useSend();
  const archive = useArchive();

  const visible = (items.data?.items ?? []).filter((item) => matches(item, chip));

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
      <div className="sg-region sg-pad flex flex-wrap items-center gap-2 py-[14px]">
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

        <span className="sg-k ml-auto">{visible.length} ITEMS</span>

        <button
          type="button"
          className="sg-chip"
          aria-label={`Switch to ${view === 'grid' ? 'index' : 'grid'} view`}
          onClick={() => setView(view === 'grid' ? 'index' : 'grid')}
        >
          {view === 'grid' ? 'GRID ↔ INDEX' : 'INDEX ↔ GRID'}
        </button>
      </div>

      {items.isPending && <p className="sg-k sg-pad py-6">LOADING…</p>}
      {items.isError && (
        <div className="sg-pad py-6">
          <p className="sg-k">LIST UNAVAILABLE</p>
          <button type="button" className="btn btn-secondary mt-2" onClick={() => items.refetch()}>
            Retry
          </button>
        </div>
      )}

      {!items.isPending && !items.isError && visible.length === 0 && (
        <p className="sg-k sg-pad py-6">NOTHING HERE YET</p>
      )}

      {view === 'grid' ? (
        <PosterGrid items={visible} admin={admin} desktop={desktop} onOpen={setOpen} />
      ) : (
        <IndexTable
          items={visible}
          admin={admin}
          desktop={desktop}
          selected={selected}
          onToggle={toggle}
          onToggleAll={toggleAll}
          onOpen={setOpen}
        />
      )}

      {selected.size > 0 && (
        <SelectionBar
          count={selected.size}
          admin={admin}
          onArchive={() => applyToSelection((item) => archive.mutate({ item, value: true }))}
          onSend={() =>
            applyToSelection((item) =>
              send.mutate({ item, value: item.media_type === 'tv' ? 'sonarr' : 'radarr' }),
            )
          }
        />
      )}

      {!desktop && <DetailSheet item={open} admin={admin} onClose={() => setOpen(null)} />}
    </>
  );
}
