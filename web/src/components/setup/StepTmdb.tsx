import type { CardProps } from '../settings/draft';
import { TextField } from '../settings/fields';
import { useTmdbTest } from '../settings/service';
import { useSaveSettings } from '../../lib/queries';
import { TestRow } from './TestRow';

export function StepTmdb({ settings, draft }: CardProps) {
  const current = { ...settings.tmdb, ...draft.patch.tmdb };
  const test = useTmdbTest();
  const save = useSaveSettings();

  /* The test reads the stored key, so the pending edit goes up first. */
  async function saveThenTest() {
    if (draft.dirty) {
      try {
        await save.mutateAsync(draft.patch);
      } catch {
        return;
      }
      draft.reset();
    }
    test.run();
  }

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
      <TestRow
        test={{
          result: test.result,
          pending: test.pending || save.isPending,
          run: () => void saveThenTest(),
        }}
      />
      <p className="m-0 text-[13px] text-muted">
        Every capture resolves through TMDB. Create a free key on themoviedb.org under Settings,
        then API.
      </p>
    </div>
  );
}
