import { Plus } from 'lucide-react';
import { useMemo, useState } from 'react';
import { useCreateService } from '../../lib/queries';
import { pushToast } from '../../lib/toast';
import type { Service, ServiceKind } from '../../lib/types';
import { Modal } from '../Modal';
import { useServiceDraft } from './draft';
import { ServiceFormBody, TestButton } from './ServiceDialog';
import { freeName, kindLabel, useTestDraft } from './service';
import { iconFor } from './ServiceTile';

/* Grouped by the job each kind does, because that is the question the reader
   arrives with — not which of the seven names they half remember. */
const GROUPS: { title: string; note: string; kinds: ServiceKind[] }[] = [
  {
    title: 'Media servers',
    note: 'What you already own. Snagarr reads these to know what is in the library.',
    kinds: ['plex', 'emby', 'jellyfin'],
  },
  {
    title: 'Download managers',
    note: 'Where a snagged title is sent.',
    kinds: ['radarr', 'sonarr'],
  },
  {
    title: 'Requests',
    note: 'Ask for a title instead of sending it straight to a download manager.',
    kinds: ['overseerr'],
  },
  {
    title: 'Notifications',
    note: 'Tells you when a title lands.',
    kinds: ['ntfy'],
  },
];

/* Nothing is written until Add. The draft is a service record that does not
   exist yet, which is why the fields below work off id 0. Test connection is
   what fills the quality profile and root folder lists: it reaches the service
   with the typed credentials, and needs no stored record either. */
function NewServiceForm({
  kind,
  services,
  onAdded,
  onBack,
  onClose,
}: {
  kind: ServiceKind;
  services: Service[];
  onAdded: (service: Service) => void;
  /** Absent when the caller chose the kind, so there is nothing to go back to. */
  onBack?: () => void;
  onClose: () => void;
}) {
  const blank = useMemo<Service>(
    () => ({
      id: 0,
      user_id: 0,
      kind,
      name: freeName(services, kind),
      config: {},
      enabled: true,
      locked: false,
      created_at: '',
      updated_at: '',
    }),
    // The name is claimed once, when the form opens. Re-deriving it as the list
    // refetches would rename what the user is typing into.
    // eslint-disable-next-line react-hooks/exhaustive-deps
    [kind],
  );

  const draft = useServiceDraft(blank);
  const create = useCreateService();
  const test = useTestDraft(kind, draft);

  return (
    <Modal
      open
      onClose={onClose}
      title={`New ${kindLabel(kind)} connection`}
      description="Nothing is saved until you select Add. Test as often as you like first."
      size="lg"
      footer={
        <>
          <button
            type="button"
            className="btn btn-primary"
            disabled={create.isPending || draft.name.trim() === ''}
            onClick={() =>
              create.mutate(
                { kind, name: draft.name.trim(), config: draft.config, enabled: draft.enabled },
                {
                  onSuccess: (service) => {
                    pushToast(`Added ${kindLabel(kind)}`);
                    onAdded(service);
                  },
                },
              )
            }
          >
            <Plus aria-hidden="true" size={16} />
            {create.isPending ? 'Adding…' : 'Add connection'}
          </button>
          <TestButton test={test} />
          <button type="button" className="btn btn-secondary ml-auto" onClick={onBack ?? onClose}>
            {onBack ? 'Back' : 'Cancel'}
          </button>
        </>
      }
    >
      <ServiceFormBody service={blank} draft={draft} test={test} />
    </Modal>
  );
}

export function AddServiceDialog({
  services,
  kind: preset,
  onAdded,
  onClose,
}: {
  services: Service[];
  /** Set by the setup wizard, which already knows which kind it is asking for. */
  kind?: ServiceKind;
  onAdded: (service: Service) => void;
  onClose: () => void;
}) {
  const [kind, setKind] = useState<ServiceKind | null>(preset ?? null);

  if (kind !== null) {
    return (
      <NewServiceForm
        kind={kind}
        services={services}
        onAdded={onAdded}
        onBack={preset ? undefined : () => setKind(null)}
        onClose={onClose}
      />
    );
  }

  return (
    <Modal
      open
      onClose={onClose}
      title="Connect a service"
      description="Pick what you are connecting. Its address and key come next."
    >
      {GROUPS.map((group) => (
        <section key={group.title} className="sg-kind-group">
          <h6 className="sg-card-section-title">{group.title}</h6>
          <p className="sg-field-help m-0">{group.note}</p>
          <div className="sg-kind-grid">
            {group.kinds.map((item) => {
              const Icon = iconFor(item);
              return (
                <button
                  key={item}
                  type="button"
                  className="sg-kind"
                  onClick={() => setKind(item)}
                >
                  <Icon aria-hidden="true" size={18} />
                  <span className="sg-kind-name">{kindLabel(item)}</span>
                </button>
              );
            })}
          </div>
        </section>
      ))}
    </Modal>
  );
}
