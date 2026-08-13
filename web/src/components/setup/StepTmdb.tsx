import type { CardProps } from '../settings/draft';
import { TextField } from '../settings/fields';
import { useSaveThenTest } from '../settings/service';
import { TestRow } from './TestRow';

export function StepTmdb({ settings, draft }: CardProps) {
  const current = { ...settings.tmdb, ...draft.patch.tmdb };
  const test = useSaveThenTest('tmdb', draft);

  return (
    <div className="flex flex-col gap-4">
      <TextField
        id="setup-tmdb"
        label="API key (v3)"
        value={current.api_key}
        locked={current.locked}
        placeholder="eyJhbGciOi…"
        onChange={(value) => draft.set('tmdb', { api_key: value })}
      />
      <TestRow test={test} />
      <p className="m-0 text-[13px] text-muted">
        Every capture resolves through TMDB. Create a free key on themoviedb.org under Settings,
        then API.
      </p>
    </div>
  );
}
