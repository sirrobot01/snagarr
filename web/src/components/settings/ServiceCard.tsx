import { useDeleteService, useSaveService } from '../../lib/queries';
import { pushToast } from '../../lib/toast';
import type { Service } from '../../lib/types';
import { useServiceDraft } from './draft';
import { CheckField, TextField } from './fields';
import { ServiceFields } from './ServiceFields';
import { cardStatus, configured, kindLabel, useSaveThenTest, type CardStatus } from './service';

interface HeadProps {
  name: string;
  configured: boolean;
  state: CardStatus['state'];
  label: string;
}

export function CardHead({ name, configured: ready, state, label }: HeadProps) {
  return (
    <div className="flex items-start gap-2">
      <span className="sg-dot mt-[6px]" data-state={state} />
      <span className="sg-card-name" data-unset={ready ? undefined : '1'}>
        {name}
      </span>
      <span className="sg-k ml-auto text-right">{label}</span>
    </div>
  );
}

export function ServiceCard({ service }: { service: Service }) {
  const draft = useServiceDraft(service);
  const save = useSaveService();
  const remove = useDeleteService();
  const test = useSaveThenTest(service, draft);

  const ready = configured(service) && !draft.dirty;
  const status = cardStatus(configured(service), test.result);
  const failed = test.result?.ok === false;

  return (
    <section className="sg-card">
      <CardHead
        name={`${kindLabel(service.kind)} · ${draft.name}`}
        configured={configured(service)}
        state={status.state}
        label={test.pending ? 'TESTING…' : status.label}
      />

      <TextField
        id={`svc-${service.id}-name`}
        label="Name"
        value={draft.name}
        locked={service.locked}
        placeholder="Default"
        onChange={draft.setName}
      />

      <ServiceFields service={service} draft={draft} ready={ready} />

      <CheckField
        id={`svc-${service.id}-enabled`}
        label="Enabled"
        checked={draft.enabled}
        locked={service.locked}
        onChange={draft.setEnabled}
      />

      {service.locked && (
        <p className="sg-k m-0">PINNED BY AN ENVIRONMENT VARIABLE — EDIT IT THERE.</p>
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
                { onSuccess: () => { draft.reset(); pushToast('SERVICE SAVED'); } },
              )
            }
          >
            {save.isPending ? 'SAVING…' : 'Save'}
          </button>
        )}
        <button
          type="button"
          className={`btn ${failed ? 'btn-primary' : 'btn-secondary'} min-h-[44px]`}
          style={{ fontSize: 12 }}
          disabled={test.pending}
          onClick={test.run}
        >
          {failed ? 'RETEST' : 'Test connection'}
        </button>
        <button
          type="button"
          className="btn btn-ghost ml-auto min-h-[44px]"
          disabled={remove.isPending || service.locked}
          onClick={() => {
            if (window.confirm(`Delete the ${kindLabel(service.kind)} service “${service.name}”?`)) {
              remove.mutate(service.id);
            }
          }}
        >
          Delete
        </button>
      </div>
    </section>
  );
}
