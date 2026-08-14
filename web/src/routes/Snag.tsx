import { useEffect, useRef, useState, type KeyboardEvent } from 'react';
import { AlertCircle, History, Keyboard, Link2, LoaderCircle, SearchX } from 'lucide-react';
import { useLocation } from 'wouter';

import { CaptureBox } from '../components/CaptureBox';
import { EmptyState } from '../components/EmptyState';
import { ItemRow } from '../components/ItemRow';
import { NeedsReview } from '../components/NeedsReview';
import { ResultRow } from '../components/ResultRow';
import { useDebounced } from '../hooks/useDebounced';
import { isUrl } from '../lib/format';
import { useCaptureQuery, useItems, useSearch, useSnag } from '../lib/queries';
import type { SearchResult } from '../lib/types';

export const SEARCH_ID = 'sg-search';

function hostOf(value: string): string {
  try {
    return new URL(value).host;
  } catch {
    return value;
  }
}

export function Snag() {
  const [query, setQuery] = useState('');
  const [focused, setFocused] = useState(false);
  const [active, setActive] = useState(0);
  const input = useRef<HTMLInputElement>(null);
  const [, navigate] = useLocation();

  const typed = query.trim();
  // A pasted link is not a search: the server dereferences it, so the box
  // switches from search results to a single capture action.
  const link = isUrl(typed);
  const debounced = useDebounced(typed, 250);
  const search = useSearch(link ? '' : debounced);
  const items = useItems(false);
  const snag = useSnag();
  const capture = useCaptureQuery();

  const results = search.data?.results ?? [];
  const idle = typed.length === 0;
  const short = typed.length < 2;
  const busy = !short && !link && (typed !== debounced || search.isFetching);

  useEffect(() => setActive(0), [debounced]);

  // Only a row that already made an item has somewhere to open, and it opens on
  // that item rather than on the list around it. A title sitting in the library
  // that nobody snagged still needs snagging.
  function select(result: SearchResult) {
    if (result.item_id !== null) {
      navigate(`/list?item=${result.item_id}`);
      return;
    }
    snag.mutate({ result, query: typed });
  }

  function snagLink() {
    if (capture.isPending) return;
    capture.mutate(typed, { onSuccess: () => setQuery('') });
  }

  function onKeyDown(event: KeyboardEvent<HTMLInputElement>) {
    if (event.key === 'Escape') {
      setQuery('');
      input.current?.blur();
      return;
    }
    if (link) {
      if (event.key === 'Enter') {
        event.preventDefault();
        snagLink();
      }
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

      {!idle && link && (
        <div className="sg-pad flex flex-col items-start gap-2 py-5">
          <p className="m-0 flex items-center gap-2 font-heading font-extrabold">
            <Link2 aria-hidden="true" size={16} /> Snag this link
          </p>
          <p className="text-muted m-0 text-[13px]">
            {hostOf(typed)} — resolved in the background. Anything ambiguous lands in Needs
            Review.
          </p>
          <button
            type="button"
            className="btn btn-secondary mt-1"
            onClick={snagLink}
            disabled={capture.isPending}
          >
            {capture.isPending && (
              <LoaderCircle className="animate-spin" aria-hidden="true" size={15} />
            )}
            {capture.isPending ? 'Snagging…' : 'Snag link'}
          </button>
          <span className="sg-k hidden items-center gap-1.5 md:flex">
            <Keyboard aria-hidden="true" size={14} /> Enter to snag
          </span>
        </div>
      )}

      {!idle && !link && (
        <>
          <div className="sg-pad flex items-center justify-between gap-3 border-b border-line py-[9px]">
            <span className="sg-k flex items-center gap-2" aria-live="polite">
              {busy && <LoaderCircle className="animate-spin" aria-hidden="true" size={13} />}
              {short
                ? 'Type at least 2 characters'
                : busy
                  ? 'Searching…'
                  : `${results.length} ${results.length === 1 ? 'result' : 'results'} · Library first`}
            </span>
            <span className="sg-k hidden items-center gap-1.5 md:flex">
              <Keyboard aria-hidden="true" size={14} /> ↑↓ Navigate · Enter to snag
            </span>
          </div>

          {search.isError && (
            <p className="sg-k sg-pad flex items-center gap-2 py-4" role="status">
              <AlertCircle aria-hidden="true" size={15} /> Search is unavailable. Retrying…
            </p>
          )}

          {!short && !busy && !search.isError && results.length === 0 && (
            <div className="sg-pad flex items-center gap-3 py-6">
              <SearchX className="text-muted" aria-hidden="true" size={20} />
              <div>
                <p className="m-0 font-heading font-extrabold">No matches for “{typed}”</p>
                <p className="text-muted m-0 text-[13px]">Try a shorter title or paste a link.</p>
              </div>
            </div>
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

          {items.isPending && (
            <p className="sg-k sg-pad flex items-center gap-2 py-6" role="status">
              <LoaderCircle className="animate-spin" aria-hidden="true" size={14} /> Loading your
              snags…
            </p>
          )}
          {items.isError && (
            <p className="sg-k sg-pad flex items-center gap-2 py-6" role="status">
              <AlertCircle aria-hidden="true" size={14} /> Your list is unavailable. Retrying…
            </p>
          )}

          {recent.length > 0 && (
            <>
              <div className="sg-section-label sg-pad">
                <History aria-hidden="true" size={15} />
                <span>Recently snagged</span>
              </div>
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
