import { ChevronRight, Plus } from 'lucide-react';
import { Badge } from './Badge';
import { Poster } from './Poster';
import type { SearchResult } from '../lib/types';

interface Props {
  result: SearchResult;
  active: boolean;
  onSelect: (result: SearchResult) => void;
}

// The badge already carries the library state, so the action only has to say
// whether the row opens an item that exists or makes a new one.
function action(result: SearchResult) {
  if (result.item_id !== null) {
    return { label: 'Open', icon: <ChevronRight aria-hidden="true" size={16} /> };
  }
  return { label: 'Snag', icon: <Plus aria-hidden="true" size={15} /> };
}

export function ResultRow({ result, active, onSelect }: Props) {
  const next = action(result);

  return (
    <button
      type="button"
      className="sg-row sg-result-row"
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
      <span className="sg-row-action">
        {next.icon}
        <span>{next.label}</span>
      </span>
    </button>
  );
}
