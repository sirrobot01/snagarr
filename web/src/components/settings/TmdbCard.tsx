import { useSaveSettings } from '../../lib/queries';
import type { CardProps } from './draft';
import { TextField } from './fields';
import { CardHead } from './ServiceCard';
import { cardStatus, useTmdbTest } from './service';

/* The one global service left. It has no service record, so it is still tested
   through POST /settings/test, which reads the stored key. */
export function TmdbCard({ settings, draft }: CardProps) {
  const current = { ...settings.tmdb, ...draft.patch.tmdb };
  const test = useTmdbTest();
  const save = useSaveSettings();
  const status = cardStatus(current.configured, test.result);
  const failed = test.result?.ok === false;
  const pending = test.pending || save.isPending;

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
    <section className="sg-card">
      <CardHead
        name="TMDB"
        configured={current.configured}
        state={status.state}
        label={pending ? 'TESTING…' : status.label}
      />

      <TextField
        id="tmdb-key"
        label="API key (v3)"
        value={current.api_key}
        locked={current.locked}
        placeholder="eyJhbGciOi…"
        onChange={(value) => draft.set('tmdb', { api_key: value })}
      />

      <button
        type="button"
        className={`btn ${failed ? 'btn-primary' : 'btn-secondary'} min-h-[44px] self-start`}
        style={{ fontSize: 12 }}
        disabled={pending}
        onClick={() => void saveThenTest()}
      >
        {failed ? 'RETEST' : 'Test connection'}
      </button>
    </section>
  );
}
