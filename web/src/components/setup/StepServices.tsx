import { useCreateService, useServices } from '../../lib/queries';
import type { ServiceKind } from '../../lib/types';
import { ServiceCard } from '../settings/ServiceCard';
import { freeName, kindLabel } from '../settings/service';
import { ErrorState, Loading } from '../settings/states';

interface Props {
  /** The kinds this step is about, in the order to offer them. */
  kinds: ServiceKind[];
  empty: string;
}

/* Both service steps are the same screen with a different set of kinds: add a
   record, then fill it in with the very card the settings page uses. */
export function StepServices({ kinds, empty }: Props) {
  const services = useServices();
  const create = useCreateService();

  if (services.isError) {
    return <ErrorState error={services.error} onRetry={() => void services.refetch()} />;
  }
  if (!services.data) return <Loading label="LOADING SERVICES…" />;

  const all = services.data.services;
  const mine = all.filter((service) => kinds.includes(service.kind));

  return (
    <div className="flex flex-col gap-5">
      {mine.map((service) => (
        <ServiceCard key={service.id} service={service} />
      ))}

      <div className="flex flex-wrap items-center gap-2">
        {kinds.map((kind) => (
          <button
            key={kind}
            type="button"
            className={`btn ${mine.length === 0 ? 'btn-primary' : 'btn-secondary'} min-h-[44px]`}
            disabled={create.isPending}
            onClick={() => create.mutate({ kind, name: freeName(all, kind) })}
          >
            Add {kindLabel(kind)}
          </button>
        ))}
      </div>

      <p className="text-muted m-0 text-[13px]">{empty}</p>
    </div>
  );
}
