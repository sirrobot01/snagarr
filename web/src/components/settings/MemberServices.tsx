import { Server, X } from 'lucide-react';
import { useUserServices } from '../../lib/queries';
import type { HouseholdUser } from '../../lib/types';
import { configured, kindLabel } from './service';
import { ErrorState, Loading } from './states';

/** An admin's read-only view of another member's stack. Editing stays with the
    owner, so this only answers "has she connected her Plex yet?". */
export function MemberServices({ user, onClose }: { user: HouseholdUser; onClose: () => void }) {
  const services = useUserServices(user.id);

  return (
    <div className="flex flex-col gap-3 border-t border-line pt-4">
      <div className="flex items-center gap-2">
        <Server aria-hidden="true" size={17} />
        <p className="sg-k m-0">{user.username}’s services</p>
        <button type="button" className="btn btn-ghost ml-auto min-h-[44px]" onClick={onClose}>
          <X aria-hidden="true" size={15} />
          Close
        </button>
      </div>

      {services.isError ? (
        <ErrorState error={services.error} onRetry={() => void services.refetch()} />
      ) : !services.data ? (
        <Loading label="Loading services…" />
      ) : services.data.services.length === 0 ? (
        <p className="text-muted m-0 text-[13px]">
          {user.username} has connected nothing yet.
        </p>
      ) : (
        <table className="table">
          <thead>
            <tr>
              <th>Service</th>
              <th>Name</th>
              <th>Address</th>
              <th>State</th>
            </tr>
          </thead>
          <tbody>
            {services.data.services.map((service) => (
              <tr key={service.id}>
                <td className="font-heading font-extrabold">{kindLabel(service.kind)}</td>
                <td className="text-muted">{service.name}</td>
                <td className="text-muted break-all">
                  {service.config.url || service.config.topic || '—'}
                </td>
                <td>
                  <span className={`sg-b ${configured(service) && service.enabled ? 'sg-lib' : 'sg-new'}`}>
                    {!service.enabled ? 'off' : configured(service) ? 'ready' : 'not set'}
                  </span>
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      )}
    </div>
  );
}
