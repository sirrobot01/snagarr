import {
  Bell,
  ChevronDown,
  Film,
  Library,
  Save,
  SendHorizontal,
  Trash2,
  Tv,
  Unplug,
  type LucideIcon,
} from 'lucide-react';
import { useState } from 'react';
import { useDeleteService, useSaveService } from '../../lib/queries';
import { pushToast } from '../../lib/toast';
import type { Service, ServiceKind } from '../../lib/types';
import { useServiceDraft } from './draft';
import { CheckField, TextField } from './fields';
import { ServiceFields } from './ServiceFields';
import { cardStatus, configured, kindLabel, useSaveThenTest, type CardStatus } from './service';

interface HeadProps {
  name: string;
  configured: boolean;
  state: CardStatus['state'];
  label: string;
  icon?: LucideIcon;
  note?: string;
}

export function CardHead({ name, configured: ready, state, label, icon: Icon, note }: HeadProps) {
  return (
    <div className="sg-card-head">
      {Icon && (
        <span className="sg-card-icon" aria-hidden="true">
          <Icon size={18} />
        </span>
      )}
      <div className="min-w-0 flex-1">
        <div className="flex items-center gap-2">
          <span className="sg-dot" data-state={state} />
          <span className="sg-card-name truncate" data-unset={ready ? undefined : '1'}>
            {name}
          </span>
        </div>
        {note && <p className="text-muted m-0 mt-1 text-[12px]">{note}</p>}
      </div>
      <span className="sg-k shrink-0 text-right">{label}</span>
    </div>
  );
}

function iconFor(kind: ServiceKind): LucideIcon {
  if (kind === 'radarr') return Film;
  if (kind === 'sonarr') return Tv;
  if (kind === 'overseerr') return SendHorizontal;
  if (kind === 'ntfy') return Bell;
  return Library;
}

export function ServiceCard({
  service,
  openOnMount = false,
}: {
  service: Service;
  openOnMount?: boolean;
}) {
  const [expanded, setExpanded] = useState(openOnMount);
  const draft = useServiceDraft(service);
  const save = useSaveService();
  const remove = useDeleteService();
  const test = useSaveThenTest(service, draft);

  const ready = configured(service) && !draft.dirty;
  const status = cardStatus(configured(service), test.result);
  const failed = test.result?.ok === false;

  return (
    <section className="sg-card" data-collapsed={expanded ? undefined : '1'}>
      <button
        type="button"
        className="sg-card-toggle"
        aria-expanded={expanded}
        aria-controls={`service-${service.id}-details`}
        onClick={() => setExpanded((open) => !open)}
      >
        <CardHead
          name={`${kindLabel(service.kind)} · ${draft.name}`}
          configured={configured(service)}
          state={status.state}
          label={test.pending ? 'Testing…' : status.label}
          icon={iconFor(service.kind)}
          note={
            !service.enabled
              ? 'Currently disabled'
              : configured(service)
                ? 'Connected'
                : 'Needs configuration'
          }
        />
        <ChevronDown className="sg-card-chevron" aria-hidden="true" size={18} />
      </button>

      {expanded && (
        <div id={`service-${service.id}-details`} className="sg-card-body">
          <div className="sg-card-fields">
            <TextField
              id={`svc-${service.id}-name`}
              label="Connection name"
              value={draft.name}
              locked={service.locked}
              placeholder="Home server"
              onChange={draft.setName}
            />

            <ServiceFields service={service} draft={draft} ready={ready} />

            <CheckField
              id={`svc-${service.id}-enabled`}
              label="Use this connection"
              checked={draft.enabled}
              locked={service.locked}
              onChange={draft.setEnabled}
            />
          </div>

          {service.locked && (
            <p className="sg-k m-0">Pinned by an environment variable — edit it there.</p>
          )}

          <div className="mt-1 flex flex-wrap items-center gap-2">
            {draft.dirty && (
              <button
                type="button"
                className="btn btn-primary min-h-[44px]"
                style={{ fontSize: 12 }}
                disabled={save.isPending}
                onClick={() =>
                  save.mutate(
                    { id: service.id, patch: draft.patch },
                    {
                      onSuccess: () => {
                        draft.reset();
                        setExpanded(false);
                        pushToast('Service saved');
                      },
                    },
                  )
                }
              >
                <Save aria-hidden="true" size={16} />
                {save.isPending ? 'Saving…' : 'Save'}
              </button>
            )}
            <button
              type="button"
              className={`btn ${failed ? 'btn-primary' : 'btn-secondary'} min-h-[44px]`}
              style={{ fontSize: 12 }}
              disabled={test.pending}
              onClick={test.run}
            >
              <Unplug aria-hidden="true" size={16} />
              {failed ? 'Retry' : 'Test connection'}
            </button>
            <button
              type="button"
              className="btn btn-ghost ml-auto min-h-[44px]"
              disabled={remove.isPending || service.locked}
              onClick={() => {
                if (
                  window.confirm(
                    `Delete the ${kindLabel(service.kind)} service “${service.name}”?`,
                  )
                ) {
                  remove.mutate(service.id);
                }
              }}
            >
              <Trash2 aria-hidden="true" size={16} />
              Delete
            </button>
          </div>
        </div>
      )}
    </section>
  );
}
