import { useState } from 'react';
import { Cable, Plus } from 'lucide-react';
import { useServices } from '../../lib/queries';
import type { Service } from '../../lib/types';
import { AddServiceDialog } from './AddServiceDialog';
import { ServiceDialog } from './ServiceDialog';
import { ServiceTile } from './ServiceTile';
import { ErrorState, Loading } from './states';

export function MyServices() {
  const services = useServices();
  const [adding, setAdding] = useState(false);
  // The record opens the dialog, but the list holds the newer copy once a save
  // has been refetched, so the fresher of the two wins below.
  const [editing, setEditing] = useState<Service | null>(null);

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
  const open = editing ? (list.find((service) => service.id === editing.id) ?? editing) : null;

  return (
    <>
      <div className="sg-toolbar sg-pad">
        <p className="sg-k m-0">
          {list.length} {list.length === 1 ? 'connection' : 'connections'}
        </p>
        <button
          type="button"
          className="btn btn-primary min-h-[44px]"
          onClick={() => setAdding(true)}
        >
          <Plus aria-hidden="true" size={16} />
          Connect a service
        </button>
      </div>

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
            <ServiceTile key={service.id} service={service} onOpen={() => setEditing(service)} />
          ))}
        </div>
      )}

      {/* The dialog collects everything before it creates anything, so a saved
          connection is already complete and needs no second pass. */}
      {adding && (
        <AddServiceDialog
          services={list}
          onAdded={() => setAdding(false)}
          onClose={() => setAdding(false)}
        />
      )}
      {open && <ServiceDialog service={open} onClose={() => setEditing(null)} />}
    </>
  );
}
