import { useMutation } from '@tanstack/react-query';
import { useState } from 'react';
import { api } from '../../lib/api';
import { useMe } from '../../lib/queries';
import { pushToast } from '../../lib/toast';
import type { ServiceKey, Settings } from '../../lib/types';
import { CopyField } from '../settings/fields';
import { errorText } from '../settings/states';

const SERVICES: { key: ServiceKey; name: string }[] = [
  { key: 'tmdb', name: 'TMDB' },
  { key: 'library', name: 'Media server' },
  { key: 'radarr', name: 'Radarr' },
  { key: 'sonarr', name: 'Sonarr' },
  { key: 'overseerr', name: 'Overseerr' },
  { key: 'ntfy', name: 'ntfy' },
  { key: 'telegram', name: 'Telegram' },
];

export function StepDone({ settings }: { settings: Settings }) {
  const me = useMe();
  const meId = me.data?.id ?? null;
  const [token, setToken] = useState<string | null>(null);

  const create = useMutation({
    mutationFn: (userId: number) => api.createToken(userId, 'Household'),
    onSuccess: (created) => setToken(created.token),
    onError: (error) => pushToast(`TOKEN FAILED — ${errorText(error).toUpperCase()}`),
  });

  return (
    <div className="flex flex-col gap-4">
      <ul className="m-0 flex list-none flex-col gap-2 p-0">
        {SERVICES.map((service) => (
          <li key={service.key} className="flex items-center gap-2">
            <span
              className="sg-dot"
              data-state={settings[service.key].configured ? 'ok' : 'unset'}
            />
            <span className="text-[14px]">{service.name}</span>
            <span className="sg-k ml-auto">
              {settings[service.key].configured ? 'CONNECTED' : 'NOT SET'}
            </span>
          </li>
        ))}
      </ul>

      <hr className="hr" style={{ margin: 0 }} />

      {token === null ? (
        <button
          type="button"
          className="btn btn-secondary min-h-[44px] self-start"
          disabled={meId === null || create.isPending}
          onClick={() => {
            if (meId !== null) create.mutate(meId);
          }}
        >
          {create.isPending ? 'GENERATING…' : 'Create a household token'}
        </button>
      ) : (
        <div className="flex flex-col gap-2">
          <CopyField id="setup-token" label="Household token" value={token} />
          <p className="m-0 sg-k">THIS TOKEN IS READABLE ONCE — COPY IT NOW.</p>
        </div>
      )}

      <ol className="m-0 flex list-decimal flex-col gap-1 pl-4 text-[13px] text-muted">
        <li>Paste the token into the iOS Shortcut and the Telegram bot.</li>
        <li>Invite the rest of the household under Settings.</li>
        <li>Snag something — the first library index runs in the background.</li>
      </ol>
    </div>
  );
}
