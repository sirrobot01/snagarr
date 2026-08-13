import { Plus } from 'lucide-react';
import { useState } from 'react';
import { useServices } from '../../lib/queries';
import type { Service, ServiceKind } from '../../lib/types';
import { AddServiceDialog } from '../settings/AddServiceDialog';
import { ServiceDialog } from '../settings/ServiceDialog';
import { ServiceTile } from '../settings/ServiceTile';
import { kindLabel } from '../settings/service';
import { ErrorState, Loading } from '../settings/states';

interface Props {
  /** The kinds this step is about, in the order to offer them. */
  kinds: ServiceKind[];
  empty: string;
}

/* Both service steps are the same screen with a different set of kinds, and
   both use the very dialogs the settings page uses. */
export function StepServices({ kinds, empty }: Props) {
  const services = useServices();
  const [adding, setAdding] = useState<ServiceKind | null>(null);
  const [editing, setEditing] = useState<Service | null>(null);

  if (services.isError) {
    return <ErrorState error={services.error} onRetry={() => void services.refetch()} />;
  }
  if (!services.data) return <Loading label="Loading services…" />;

  const all = services.data.services;
  const mine = all.filter((service) => kinds.includes(service.kind));
  const open = editing ? (all.find((service) => service.id === editing.id) ?? editing) : null;

  return (
    <div className="flex flex-col gap-5">
      {mine.length > 0 && (
        <div className="sg-cards">
          {mine.map((service) => (
            <ServiceTile key={service.id} service={service} onOpen={() => setEditing(service)} />
          ))}
        </div>
      )}

      <div className="flex flex-wrap items-center gap-2">
        {kinds.map((kind) => (
          <button
            key={kind}
            type="button"
            className={`btn ${mine.length === 0 ? 'btn-primary' : 'btn-secondary'} min-h-[44px]`}
            onClick={() => setAdding(kind)}
          >
            <Plus aria-hidden="true" size={16} />
            Add {kindLabel(kind)}
          </button>
        ))}
      </div>

      <p className="text-muted m-0 text-[13px]">{empty}</p>

      {adding && (
        <AddServiceDialog
          services={all}
          kind={adding}
          onAdded={() => setAdding(null)}
          onClose={() => setAdding(null)}
        />
      )}
      {open && <ServiceDialog service={open} onClose={() => setEditing(null)} />}
    </div>
  );
}
