import { useUserServices } from '../../lib/queries';
import type { HouseholdUser } from '../../lib/types';
import { Modal } from '../Modal';
import { configured, kindLabel } from './service';
import { ErrorState, Loading } from './states';

/** An admin's read-only view of another member's stack. Editing stays with the
    owner, so this only answers "has she connected her Plex yet?". */
export function MemberServicesDialog({
  user,
  onClose,
}: {
  user: HouseholdUser;
  onClose: () => void;
}) {
  const services = useUserServices(user.id);

  return (
    <Modal
      open
      onClose={onClose}
      title={`Services · ${user.username}`}
      description="Read only. Each member connects and edits their own services."
      size="lg"
      footer={
        <button type="button" className="btn btn-secondary ml-auto" onClick={onClose}>
          Done
        </button>
      }
    >
      {services.isError ? (
        <ErrorState error={services.error} onRetry={() => void services.refetch()} />
      ) : !services.data ? (
        <Loading label="Loading services…" />
      ) : services.data.services.length === 0 ? (
        <p className="text-muted m-0 text-[13px]">{user.username} has connected nothing yet.</p>
      ) : (
        <div className="sg-table-wrap">
          <table className="table sg-table-stack">
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
                  <td data-label="Service" className="font-heading font-extrabold">
                    {kindLabel(service.kind)}
                  </td>
                  <td data-label="Name" className="text-muted">
                    {service.name}
                  </td>
                  <td data-label="Address" className="text-muted break-all">
                    {service.config.url || service.config.topic || '—'}
                  </td>
                  <td data-label="State">
                    <span
                      className={`sg-b ${configured(service) && service.enabled ? 'sg-lib' : 'sg-new'}`}
                    >
                      {!service.enabled ? 'off' : configured(service) ? 'ready' : 'not set'}
                    </span>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}
    </Modal>
  );
}
