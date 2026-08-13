import { useMutation } from '@tanstack/react-query';
import { useState } from 'react';
import { api } from '../../lib/api';
import { useMe, useStatus } from '../../lib/queries';
import { pushToast } from '../../lib/toast';
import { CopyField } from '../settings/fields';
import { errorText } from '../settings/states';

/* /status reports what the household as a whole can reach, which is the right
   question now that each integration belongs to one member. */
const SERVICES: { key: string; name: string }[] = [
  { key: 'tmdb', name: 'TMDB' },
  { key: 'library', name: 'Media server' },
  { key: 'radarr', name: 'Radarr' },
  { key: 'sonarr', name: 'Sonarr' },
  { key: 'overseerr', name: 'Overseerr' },
  { key: 'ntfy', name: 'ntfy' },
];

export function StepDone() {
  const me = useMe();
  const status = useStatus();
  const meId = me.data?.id ?? null;
  const [token, setToken] = useState<string | null>(null);
  const reachable = status.data?.services ?? {};

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
            <span className="sg-dot" data-state={reachable[service.key] ? 'ok' : 'unset'} />
            <span className="text-[14px]">{service.name}</span>
            <span className="sg-k ml-auto">
              {reachable[service.key] ? 'CONNECTED' : 'NOT SET'}
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
        <li>Send each member their Apple Shortcut from Settings.</li>
        <li>Invite the rest of the household under Settings.</li>
        <li>Snag something — the first library index runs in the background.</li>
      </ol>
    </div>
  );
}
