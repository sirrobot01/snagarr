import { Check, Circle, Search, Settings, Users } from 'lucide-react';
import { useStatus } from '../../lib/queries';

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
  const status = useStatus();
  const reachable = status.data?.services ?? {};

  return (
    <div className="flex flex-col gap-4">
      <ul className="m-0 flex list-none flex-col gap-2 p-0">
        {SERVICES.map((service) => (
          <li key={service.key} className="flex items-center gap-2">
            {reachable[service.key] ? (
              <Check className="sg-success-icon" aria-hidden="true" size={16} />
            ) : (
              <Circle className="text-muted" aria-hidden="true" size={16} />
            )}
            <span className="text-[14px]">{service.name}</span>
            <span className="sg-k ml-auto">
              {reachable[service.key] ? 'Connected' : 'Not set'}
            </span>
          </li>
        ))}
      </ul>

      <hr className="hr" style={{ margin: 0 }} />

      <div className="sg-success flex items-center gap-2 p-3">
        <Check aria-hidden="true" size={17} />
        Your account is ready. You’ll stay signed in on this device.
      </div>

      <ul className="sg-next-steps">
        <li>
          <Search aria-hidden="true" size={17} />
          <span>Search for a movie or series and add your first snag.</span>
        </li>
        <li>
          <Users aria-hidden="true" size={17} />
          <span>Invite household members from Settings when you’re ready.</span>
        </li>
        <li>
          <Settings aria-hidden="true" size={17} />
          <span>Add optional capture tools and integrations later in Settings.</span>
        </li>
      </ul>
    </div>
  );
}
