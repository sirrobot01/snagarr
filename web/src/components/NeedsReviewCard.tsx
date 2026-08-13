import { useState } from 'react';
import { Poster } from './Poster';
import { shortDate } from '../lib/format';
import { useArchive, useItem, useResolve } from '../lib/queries';
import type { Candidate, Item } from '../lib/types';

interface Props {
  item: Item;
  onSearchManually: (raw: string) => void;
}

export function NeedsReviewCard({ item, onSearchManually }: Props) {
  const detail = useItem(item.id);
  const resolve = useResolve();
  const archive = useArchive();
  const [picked, setPicked] = useState<number | null>(null);

  const candidates = detail.data?.candidates ?? [];
  const selected: Candidate | undefined =
    candidates.find((c) => c.tmdb_id === picked) ?? candidates[0];

  return (
    <div className="sg-pad py-4">
      <p className="sg-k">
        {['CAPTURED ' + shortDate(item.captured_at), item.source, item.captured_by?.display_name]
          .filter(Boolean)
          .join(' · ')
          .toUpperCase()}
      </p>
      <p className="sg-raw mt-1">&ldquo;{item.raw_input}&rdquo;</p>

      {detail.isPending && <p className="sg-k mt-4">LOADING CANDIDATES…</p>}
      {detail.isError && <p className="sg-k mt-4">CANDIDATES UNAVAILABLE</p>}

      {candidates.length > 0 && (
        <div className="sg-cand-grid mt-4">
          {candidates.slice(0, 3).map((candidate) => (
            <button
              key={`${candidate.media_type}-${candidate.tmdb_id}`}
              type="button"
              className="sg-cand"
              data-selected={candidate.tmdb_id === selected?.tmdb_id ? '1' : undefined}
              aria-pressed={candidate.tmdb_id === selected?.tmdb_id}
              onClick={() => setPicked(candidate.tmdb_id)}
            >
              <Poster fill path={candidate.poster_path} title={candidate.title} size="w185" />
              <span className="sg-cand-title block">{candidate.title}</span>
              <span className="sg-cand-sub block">
                {candidate.year ?? '—'} · {Math.round(candidate.score * 100)}% match
              </span>
            </button>
          ))}
        </div>
      )}

      <div className="mt-4 flex flex-wrap items-center gap-2">
        <button
          type="button"
          className="btn btn-primary"
          disabled={!selected || resolve.isPending}
          onClick={() => selected && resolve.mutate({ item, candidate: selected })}
        >
          {selected
            ? `CONFIRM ${selected.title.toUpperCase()}${selected.year ? ` (${selected.year})` : ''}`
            : 'CONFIRM'}
        </button>

        <button
          type="button"
          className="btn btn-secondary"
          onClick={() => onSearchManually(item.raw_input)}
        >
          Search manually
        </button>

        <button
          type="button"
          className="btn btn-ghost ml-auto"
          onClick={() => archive.mutate({ item, value: true })}
        >
          Discard
        </button>
      </div>
    </div>
  );
}
