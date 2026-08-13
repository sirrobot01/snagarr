import type { Service, ServiceConfig } from '../../lib/types';
import type { ServiceDraft } from './draft';
import { CheckField, TextField } from './fields';
import { ArrOptionFields, SectionsField } from './options';
import { PlexSignIn } from './PlexSignIn';
import { isArr, isLibrary } from './service';

interface Props {
  service: Service;
  draft: ServiceDraft;
  /** True once the stored record has working credentials to look options up with. */
  ready: boolean;
}

const ARR_URL = { radarr: 'http://radarr.lan:7878', sonarr: 'http://sonarr.lan:8989' };

const LIBRARY_URL = {
  plex: 'http://plex.lan:32400',
  emby: 'http://emby.lan:8096',
  jellyfin: 'http://jellyfin.lan:8096',
};

/** Renders the fields the kind's config document actually has. */
export function ServiceFields({ service, draft, ready }: Props) {
  const { id, kind, locked } = service;
  const config = draft.config;
  const set = (values: ServiceConfig) => draft.set(values);

  if (isLibrary(kind)) {
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
        {kind === 'plex' && !locked && (
          <PlexSignIn onPicked={(url, token) => set({ url, token })} />
        )}
        <SectionsField
          id={id}
          config={config}
          locked={locked}
          ready={ready}
          onChange={set}
        />
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

  if (isArr(kind)) {
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

  if (kind === 'ntfy') {
    return (
      <>
        <TextField
          id={`svc-${id}-url`}
          label="Server"
          value={config.url ?? ''}
          locked={locked}
          inputMode="url"
          placeholder="https://ntfy.sh"
          onChange={(value) => set({ url: value })}
        />
        <TextField
          id={`svc-${id}-topic`}
          label="Topic"
          value={config.topic ?? ''}
          locked={locked}
          placeholder="snagarr-home"
          onChange={(value) => set({ topic: value })}
        />
        <TextField
          id={`svc-${id}-token`}
          label="Token (optional)"
          value={config.token ?? ''}
          locked={locked}
          onChange={(value) => set({ token: value })}
        />
      </>
    );
  }

  return (
    <>
      <TextField
        id={`svc-${id}-url`}
        label="URL"
        value={config.url ?? ''}
        locked={locked}
        inputMode="url"
        placeholder="http://overseerr.lan:5055"
        onChange={(value) => set({ url: value })}
      />
      <TextField
        id={`svc-${id}-key`}
        label="API key"
        value={config.api_key ?? ''}
        locked={locked}
        onChange={(value) => set({ api_key: value })}
      />
    </>
  );
}
