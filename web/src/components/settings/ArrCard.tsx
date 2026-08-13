import type { Draft } from './draft';
import { TextField } from './fields';
import { ArrOptionFields } from './options';
import { ServiceCard } from './ServiceCard';
import type { Settings } from '../../lib/types';

interface Props {
  service: 'radarr' | 'sonarr';
  name: string;
  settings: Settings;
  draft: Draft;
}

export function ArrCard({ service, name, settings, draft }: Props) {
  const current = { ...settings[service], ...draft.patch[service] };

  return (
    <ServiceCard service={service} name={name} configured={current.configured} draft={draft}>
      <TextField
        id={`${service}-url`}
        label="URL"
        value={current.url}
        locked={current.locked}
        inputMode="url"
        placeholder={service === 'radarr' ? 'http://radarr.lan:7878' : 'http://sonarr.lan:8989'}
        onChange={(value) => draft.set(service, { url: value })}
      />
      <TextField
        id={`${service}-key`}
        label="API key"
        value={current.api_key}
        locked={current.locked}
        onChange={(value) => draft.set(service, { api_key: value })}
      />
      <ArrOptionFields
        service={service}
        current={current}
        onChange={(values) => draft.set(service, values)}
      />
    </ServiceCard>
  );
}
