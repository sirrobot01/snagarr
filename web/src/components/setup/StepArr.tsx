import type { Settings } from '../../lib/types';
import type { CardProps, Draft } from '../settings/draft';
import { TextField } from '../settings/fields';
import { ArrOptionFields } from '../settings/options';
import { cardStatus, useSaveThenTest } from '../settings/service';
import { TestRow } from './TestRow';

interface BlockProps {
  service: 'radarr' | 'sonarr';
  name: string;
  settings: Settings;
  draft: Draft;
}

function ArrBlock({ service, name, settings, draft }: BlockProps) {
  const current = { ...settings[service], ...draft.patch[service] };
  const test = useSaveThenTest(service, draft);
  const status = cardStatus(current.configured, test.result);

  return (
    <section className="flex flex-col gap-3">
      <div className="flex items-center gap-2">
        <span className="sg-dot" data-state={status.state} />
        <span className="sg-card-name" data-unset={current.configured ? undefined : '1'}>
          {name}
        </span>
        <span className="sg-k ml-auto">{test.pending ? 'TESTING…' : status.label}</span>
      </div>

      <TextField
        id={`setup-${service}-url`}
        label="URL"
        value={current.url}
        locked={current.locked}
        inputMode="url"
        placeholder={service === 'radarr' ? 'http://radarr.lan:7878' : 'http://sonarr.lan:8989'}
        onChange={(value) => draft.set(service, { url: value })}
      />
      <TextField
        id={`setup-${service}-key`}
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
      <TestRow test={test} />
    </section>
  );
}

export function StepArr({ settings, draft }: CardProps) {
  return (
    <div className="flex flex-col gap-6">
      <ArrBlock service="radarr" name="Radarr" settings={settings} draft={draft} />
      <hr className="hr" style={{ margin: 0 }} />
      <ArrBlock service="sonarr" name="Sonarr" settings={settings} draft={draft} />
      <p className="m-0 text-[13px] text-muted">
        Test each service to load its quality profiles and root folders. Skip this step if you send
        everything through Overseerr.
      </p>
    </div>
  );
}
