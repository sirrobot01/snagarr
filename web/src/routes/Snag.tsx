import { useEffect, useRef, useState, type KeyboardEvent } from 'react';
import { useLocation } from 'wouter';

import { CaptureBox } from '../components/CaptureBox';
import { EmptyState } from '../components/EmptyState';
import { ItemRow } from '../components/ItemRow';
import { NeedsReview } from '../components/NeedsReview';
import { ResultRow } from '../components/ResultRow';
import { useDebounced } from '../hooks/useDebounced';
import { useItems, useSearch, useSnag } from '../lib/queries';
import type { SearchResult } from '../lib/types';

export const SEARCH_ID = 'sg-search';

export function Snag() {
  const [query, setQuery] = useState('');
  const [focused, setFocused] = useState(false);
  const [active, setActive] = useState(0);
  const input = useRef<HTMLInputElement>(null);
  const [, navigate] = useLocation();

  const debounced = useDebounced(query.trim(), 250);
  const search = useSearch(debounced);
  const items = useItems(false);
  const snag = useSnag();

  const results = search.data?.results ?? [];
  const typed = query.trim();
  const idle = typed.length === 0;
  const short = typed.length < 2;
  const busy = !short && (typed !== debounced || search.isFetching);

  useEffect(() => setActive(0), [debounced]);

  function select(result: SearchResult) {
    if (result.state === 'available' || result.state === 'watched' || result.item_id !== null) {
      navigate('/list');
      return;
    }
    snag.mutate({ result, query: typed });
  }

  function onKeyDown(event: KeyboardEvent<HTMLInputElement>) {
    if (event.key === 'Escape') {
      setQuery('');
      input.current?.blur();
      return;
    }
    if (results.length === 0) return;

    if (event.key === 'ArrowDown') {
      event.preventDefault();
      setActive((i) => (i + 1) % results.length);
    } else if (event.key === 'ArrowUp') {
      event.preventDefault();
      setActive((i) => (i - 1 + results.length) % results.length);
    } else if (event.key === 'Enter') {
      event.preventDefault();
      const result = results[active];
      if (result) select(result);
    }
  }

  const needsReview = (items.data?.items ?? []).filter((item) => item.status === 'needs_review');
  const recent = (items.data?.items ?? []).filter((item) => item.status !== 'needs_review');

  return (
    <>
      <CaptureBox
        id={SEARCH_ID}
        ref={input}
        value={query}
        onChange={setQuery}
        onKeyDown={onKeyDown}
        focused={focused}
        onFocusChange={setFocused}
      />

      {!idle && (
        <>
          <div className="sg-pad flex items-center justify-between gap-3 border-b border-line py-[9px]">
            <span className="sg-k">
              {short
                ? 'KEEP TYPING'
                : busy
                  ? 'SEARCHING…'
                  : `${results.length} RESULTS · LIBRARY FIRST`}
            </span>
            <span className="sg-k hidden md:inline">↑↓ MOVE · ⏎ SNAG</span>
          </div>

          {search.isError && <p className="sg-k sg-pad py-4">SEARCH UNAVAILABLE — RETRYING</p>}

          {!short && !busy && !search.isError && results.length === 0 && (
            <p className="sg-k sg-pad py-4">NO MATCHES</p>
          )}

          {results.map((result, index) => (
            <ResultRow
              key={`${result.media_type}-${result.tmdb_id}`}
              result={result}
              active={index === active}
              onSelect={select}
            />
          ))}
        </>
      )}

      {idle && (
        <>
          <NeedsReview items={needsReview} onSearchManually={setQuery} />

          {items.isPending && <p className="sg-k sg-pad py-6">LOADING…</p>}
          {items.isError && <p className="sg-k sg-pad py-6">LIST UNAVAILABLE — RETRYING</p>}

          {recent.length > 0 && (
            <>
              <p className="sg-k sg-pad border-b border-line py-[9px]">RECENT SNAGS</p>
              {recent.slice(0, 12).map((item) => (
                <ItemRow key={item.id} item={item} onSelect={() => navigate('/list')} />
              ))}
            </>
          )}

          {!items.isPending && !items.isError && recent.length === 0 && needsReview.length === 0 && (
            <EmptyState />
          )}
        </>
      )}
    </>
  );
}
