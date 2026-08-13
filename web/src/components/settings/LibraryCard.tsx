import type { CardProps } from './draft';
import { Seg, TextField } from './fields';
import { PROVIDERS, SectionsField } from './options';
import { ServiceCard } from './ServiceCard';

export function LibraryCard({ settings, draft }: CardProps) {
  const current = { ...settings.library, ...draft.patch.library };

  return (
    <ServiceCard service="library" name="Media server" configured={current.configured} draft={draft}>
      <Seg
        name="library-provider-card"
        value={current.provider}
        options={PROVIDERS}
        locked={current.locked}
        onChange={(value) => draft.set('library', { provider: value })}
      />
      <TextField
        id="library-url"
        label="URL"
        value={current.url}
        locked={current.locked}
        inputMode="url"
        placeholder="http://plex.lan:32400"
        onChange={(value) => draft.set('library', { url: value })}
      />
      <TextField
        id="library-token"
        label="Token"
        value={current.token}
        locked={current.locked}
        onChange={(value) => draft.set('library', { token: value })}
      />
      <SectionsField
        current={current}
        onChange={(values) => draft.set('library', values)}
      />
      <TextField
        id="library-collection"
        label="Collection name"
        value={current.collection_name}
        locked={current.locked}
        placeholder="Snagged"
        onChange={(value) => draft.set('library', { collection_name: value })}
      />
    </ServiceCard>
  );
}
