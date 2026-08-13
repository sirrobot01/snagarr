import type { CardProps } from '../settings/draft';
import { Seg, TextField } from '../settings/fields';
import { PROVIDERS, SectionsField } from '../settings/options';
import { useSaveThenTest, useServiceOptions } from '../settings/service';
import { TestRow } from './TestRow';

function libraryNote(message: string, sections: number): string {
  const match = /([\d,]+)\s*items?/i.exec(message);
  if (sections === 0 || match === null) return `CONNECTED — ${message.toUpperCase()}.`;

  const items = match[1];
  const seconds = Math.max(5, Math.round(Number(items.replace(/,/g, '')) / 1000) * 10);
  return (
    `CONNECTED — ${sections} SECTION${sections === 1 ? '' : 'S'}, ${items} ITEMS.` +
    ` FIRST INDEX WILL TAKE ~${seconds} S.`
  );
}

export function StepLibrary({ settings, draft }: CardProps) {
  const current = { ...settings.library, ...draft.patch.library };
  const test = useSaveThenTest('library', draft);
  const options = useServiceOptions('library', current.configured);
  const sections = options.data?.sections ?? [];
  const connected = test.result?.ok === true;

  return (
    <div className="flex flex-col gap-4">
      <Seg
        name="setup-library-provider"
        value={current.provider}
        options={PROVIDERS}
        locked={current.locked}
        onChange={(value) => draft.set('library', { provider: value })}
      />
      <TextField
        id="setup-library-url"
        label="URL"
        value={current.url}
        locked={current.locked}
        inputMode="url"
        placeholder="http://plex.lan:32400"
        onChange={(value) => draft.set('library', { url: value })}
      />
      <TextField
        id="setup-library-token"
        label="Token"
        value={current.token}
        locked={current.locked}
        onChange={(value) => draft.set('library', { token: value })}
      />

      <TestRow
        test={test}
        note={test.result ? libraryNote(test.result.message, sections.length) : undefined}
      />

      {connected && sections.length > 0 && (
        <SectionsField current={current} onChange={(values) => draft.set('library', values)} />
      )}
    </div>
  );
}
