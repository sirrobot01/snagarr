import { useServices } from '../../lib/queries';
import { AddService } from './AddService';
import { ServiceCard } from './ServiceCard';
import { ErrorState, Loading } from './states';

export function MyServices() {
  const services = useServices();

  if (services.isError) {
    return (
      <div className="sg-pad">
        <ErrorState error={services.error} onRetry={() => void services.refetch()} />
      </div>
    );
  }
  if (!services.data) {
    return (
      <div className="sg-pad">
        <Loading label="LOADING SERVICES…" />
      </div>
    );
  }

  const list = services.data.services;

  return (
    <>
      {list.length === 0 ? (
        <p className="sg-pad text-muted m-0 py-4 text-[13px]">
          No services yet. Add your media server first, then Radarr or Sonarr.
        </p>
      ) : (
        <div className="sg-cards">
          {list.map((service) => (
            <ServiceCard key={service.id} service={service} />
          ))}
        </div>
      )}
      <AddService services={list} />
    </>
  );
}
