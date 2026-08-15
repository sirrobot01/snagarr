import { ExternalLink } from 'lucide-react';
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
        label={current.builtin_key ? 'API key (v3) — optional' : 'API key (v3)'}
        value={current.api_key}
        locked={current.locked}
        type="password"
        placeholder={
          current.builtin_key ? 'Using Snagarr’s built-in key' : 'Paste your TMDB API key'
        }
        description={
          current.builtin_key
            ? 'Snagarr ships with a shared TMDB key, so you can continue without one. Paste your own key to use it instead.'
            : 'The key is stored securely and used only to look up title metadata.'
        }
        onChange={(value) => draft.set('tmdb', { api_key: value })}
      />
      <TestRow
        test={{
          result: test.result,
          pending: test.pending || save.isPending,
          run: () => void saveThenTest(),
          // TMDB is a settings field, not a service, so it offers no options.
          probed: null,
        }}
      />
      <a
        className="inline-flex items-center gap-1.5 self-start text-[13px]"
        href="https://www.themoviedb.org/settings/api"
        target="_blank"
        rel="noreferrer"
      >
        Get a free key from TMDB <ExternalLink aria-hidden="true" size={14} />
      </a>
    </div>
  );
}
