import { useState } from 'react';
import { Cable } from 'lucide-react';
import { useServices } from '../../lib/queries';
import { AddService } from './AddService';
import { ServiceCard } from './ServiceCard';
import { ErrorState, Loading } from './states';

export function MyServices() {
  const services = useServices();
  // A service is created empty, so the card it adds opens on its own fields
  // rather than making the user hunt for them.
  const [added, setAdded] = useState<number | null>(null);

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
        <Loading label="Loading services…" />
      </div>
    );
  }

  const list = services.data.services;

  return (
    <>
      {list.length === 0 ? (
        <div className="sg-empty sg-pad flex flex-col items-center py-8 text-center">
          <span className="sg-empty-icon" aria-hidden="true">
            <Cable size={24} />
          </span>
          <h4 className="mt-3">No services connected</h4>
          <p className="text-muted m-0 max-w-[340px] text-[13px]">
            Start with your media server, then add Radarr or Sonarr when you’re ready.
          </p>
        </div>
      ) : (
        <div className="sg-cards">
          {list.map((service) => (
            <ServiceCard key={service.id} service={service} openOnMount={service.id === added} />
          ))}
        </div>
      )}
      <AddService services={list} onAdded={setAdded} />
    </>
  );
}
