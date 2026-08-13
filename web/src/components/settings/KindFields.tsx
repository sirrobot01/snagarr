import type { ArrKind, LibraryKind, ServiceConfig } from '../../lib/types';
import { CheckField, TextField } from './fields';
import { ArrOptionFields, SectionsField } from './options';
import { PlexSignIn } from './PlexSignIn';

interface Props<K> {
  id: number;
  kind: K;
  config: ServiceConfig;
  locked: boolean;
  ready: boolean;
  set: (values: ServiceConfig) => void;
}

const LIBRARY_URL: Record<LibraryKind, string> = {
  plex: 'http://plex.lan:32400',
  emby: 'http://emby.lan:8096',
  jellyfin: 'http://jellyfin.lan:8096',
};

const ARR_URL: Record<ArrKind, string> = {
  radarr: 'http://radarr.lan:7878',
  sonarr: 'http://sonarr.lan:8989',
};

export function LibraryFields({ id, kind, config, locked, ready, set }: Props<LibraryKind>) {
  return (
    <>
      <TextField
        id={`svc-${id}-url`}
        label="URL"
        value={config.url ?? ''}
        locked={locked}
        inputMode="url"
        placeholder={LIBRARY_URL[kind]}
        onChange={(value) => set({ url: value })}
      />
      <TextField
        id={`svc-${id}-token`}
        label="Token"
        value={config.token ?? ''}
        locked={locked}
        onChange={(value) => set({ token: value })}
      />
      {/* Plex can hand the token over itself, which beats hunting for one in
          the XML of a library page. */}
      {kind === 'plex' && !locked && (
        <PlexSignIn onPicked={(url, token) => set({ url, token })} />
      )}
      <SectionsField id={id} config={config} locked={locked} ready={ready} onChange={set} />
      <TextField
        id={`svc-${id}-collection`}
        label="Collection name"
        value={config.collection_name ?? ''}
        locked={locked}
        placeholder="Snagged"
        onChange={(value) => set({ collection_name: value })}
      />
    </>
  );
}

export function ArrFields({ id, kind, config, locked, ready, set }: Props<ArrKind>) {
  return (
    <>
      <TextField
        id={`svc-${id}-url`}
        label="URL"
        value={config.url ?? ''}
        locked={locked}
        inputMode="url"
        placeholder={ARR_URL[kind]}
        onChange={(value) => set({ url: value })}
      />
      <TextField
        id={`svc-${id}-key`}
        label="API key"
        value={config.api_key ?? ''}
        locked={locked}
        onChange={(value) => set({ api_key: value })}
      />
      <ArrOptionFields
        id={id}
        kind={kind}
        config={config}
        locked={locked}
        ready={ready}
        onChange={set}
      />
      <CheckField
        id={`svc-${id}-search`}
        label="Search on add"
        checked={config.search_on_add === true}
        locked={locked}
        onChange={(checked) => set({ search_on_add: checked })}
      />
      {kind === 'sonarr' && (
        <CheckField
          id={`svc-${id}-season`}
          label="Use season folders"
          checked={config.season_folder === true}
          locked={locked}
          onChange={(checked) => set({ season_folder: checked })}
        />
      )}
    </>
  );
}
