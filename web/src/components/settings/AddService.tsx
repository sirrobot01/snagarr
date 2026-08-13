import { useState } from 'react';
import { Plus, PlugZap } from 'lucide-react';
import { useCreateService } from '../../lib/queries';
import { pushToast } from '../../lib/toast';
import type { Service, ServiceKind } from '../../lib/types';
import { KINDS, freeName, kindLabel } from './service';

/* A new service is created straight away with its kind's defaults. That gives
   it an ID, which is what the test and options endpoints hang off. */
export function AddService({
  services,
  onAdded,
}: {
  services: Service[];
  onAdded: (id: number) => void;
}) {
  const [kind, setKind] = useState<ServiceKind>('plex');
  const create = useCreateService();

  return (
    <div className="sg-add-service sg-pad">
      <div className="sg-section-heading">
        <PlugZap aria-hidden="true" size={18} />
        <div>
          <h5 className="m-0">Connect another service</h5>
          <p className="text-muted m-0 text-[12px]">
            Add a media server, download manager, request service, or notification channel.
          </p>
        </div>
      </div>
      <div className="field min-w-[190px]">
        <label htmlFor="add-service-kind">Add service</label>
        <select
          id="add-service-kind"
          className="input"
          style={{ minHeight: 44 }}
          value={kind}
          onChange={(event) => setKind(event.target.value as ServiceKind)}
        >
          {KINDS.map((item) => (
            <option key={item.value} value={item.value}>
              {item.label}
            </option>
          ))}
        </select>
      </div>
      <button
        type="button"
        className="btn btn-primary min-h-[44px]"
        disabled={create.isPending}
        onClick={() =>
          create.mutate(
            { kind, name: freeName(services, kind) },
            {
              onSuccess: (service) => {
                onAdded(service.id);
                pushToast(`Added ${kindLabel(kind)}`);
              },
            },
          )
        }
      >
        <Plus aria-hidden="true" size={16} />
        {create.isPending ? 'Adding…' : 'Add'}
      </button>
    </div>
  );
}
