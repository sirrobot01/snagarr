import type { Service, ServiceConfig } from '../../lib/types';
import type { ServiceDraft } from './draft';
import { CheckField, TextField } from './fields';
import { ArrFields, LibraryFields } from './KindFields';
import { isArr, isLibrary } from './service';

interface Props {
  service: Service;
  draft: ServiceDraft;
  /** True once the stored record has working credentials to look options up with. */
  ready: boolean;
}

/** Renders the fields the kind's config document actually has. */
export function ServiceFields({ service, draft, ready }: Props) {
  const { id, kind, locked } = service;
  const config = draft.config;
  const set = (values: ServiceConfig) => draft.set(values);

  if (isLibrary(kind)) {
    return <LibraryFields id={id} kind={kind} config={config} locked={locked} ready={ready} set={set} />;
  }
  if (isArr(kind)) {
    return <ArrFields id={id} kind={kind} config={config} locked={locked} ready={ready} set={set} />;
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
          type="url"
          placeholder="https://ntfy.sh"
          description="Your ntfy server address. The hosted service is used by default."
          onChange={(value) => set({ url: value })}
        />
        <TextField
          id={`svc-${id}-topic`}
          label="Topic"
          value={config.topic ?? ''}
          locked={locked}
          placeholder="snagarr-home"
          description="The topic that receives Snagarr notifications."
          onChange={(value) => set({ topic: value })}
        />
        <TextField
          id={`svc-${id}-token`}
          label="Token (optional)"
          value={config.token ?? ''}
          locked={locked}
          type="password"
          onChange={(value) => set({ token: value })}
        />
        <CheckField
          id={`svc-${id}-priority`}
          label="Send at high priority"
          checked={(config.priority ?? 3) > 3}
          locked={locked}
          onChange={(checked) => set({ priority: checked ? 4 : 3 })}
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
        type="url"
        placeholder="http://overseerr.lan:5055"
        description="The local or public address of your Overseerr server."
        onChange={(value) => set({ url: value })}
      />
      <TextField
        id={`svc-${id}-key`}
        label="API key"
        value={config.api_key ?? ''}
        locked={locked}
        type="password"
        description="Find this under Settings → General in Overseerr."
        onChange={(value) => set({ api_key: value })}
      />
    </>
  );
}
