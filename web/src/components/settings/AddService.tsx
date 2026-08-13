import { useState } from 'react';
import { useCreateService } from '../../lib/queries';
import { pushToast } from '../../lib/toast';
import type { Service, ServiceKind } from '../../lib/types';
import { KINDS, freeName, kindLabel } from './service';

/* A new service is created straight away with its kind's defaults. That gives
   it an ID, which is what the test and options endpoints hang off. */
export function AddService({ services }: { services: Service[] }) {
  const [kind, setKind] = useState<ServiceKind>('plex');
  const create = useCreateService();

  return (
    <div className="sg-pad flex flex-wrap items-end gap-2 py-4">
      <div className="field">
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
            { onSuccess: () => pushToast(`ADDED — ${kindLabel(kind).toUpperCase()}`) },
          )
        }
      >
        {create.isPending ? 'ADDING…' : 'Add'}
      </button>
    </div>
  );
}
