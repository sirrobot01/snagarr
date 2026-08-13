import { Badge } from './Badge';
import { Poster } from './Poster';
import type { SearchResult } from '../lib/types';

interface Props {
  result: SearchResult;
  active: boolean;
  onSelect: (result: SearchResult) => void;
}

function action(result: SearchResult): string {
  if (result.state === 'available' || result.state === 'watched') return 'OPEN';
  if (result.item_id !== null) return '✓';
  return 'SNAG';
}

export function ResultRow({ result, active, onSelect }: Props) {
  const label = action(result);

  return (
    <button
      type="button"
      className="sg-row"
      data-active={active ? '1' : undefined}
      onClick={() => onSelect(result)}
    >
      <Poster
        className="sg-row-poster"
        path={result.poster_path}
        title={result.title}
        size="w185"
      />

      <span className="min-w-0 flex-1">
        <span className="sg-row-title block truncate">{result.title}</span>
        <span className="sg-row-meta block truncate">
          {[
            result.year,
            result.media_type === 'tv' ? 'Series' : 'Movie',
            result.from === 'library' ? 'Local index' : 'TMDB',
          ]
            .filter(Boolean)
            .join(' · ')}
        </span>
      </span>

      <Badge state={result.state} />
      <span className="sg-row-action">{label}</span>
    </button>
  );
}
