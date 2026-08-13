import { Save, Trash2, Unplug } from 'lucide-react';
import { useState } from 'react';
import { useDeleteService, useSaveService } from '../../lib/queries';
import { pushToast } from '../../lib/toast';
import type { Service } from '../../lib/types';
import { ConfirmDialog, Modal } from '../Modal';
import type { ServiceDraft } from './draft';
import { useServiceDraft } from './draft';
import { CheckField, TextField } from './fields';
import { ServiceFields } from './ServiceFields';
import {
  cardStatus,
  configured,
  kindLabel,
  useServiceOptions,
  useTestDraft,
  type ServiceTest,
} from './service';

/** The fields of one connection. Shared by the dialog that edits a stored
    service and the one that builds a new one, because they ask for the same
    things in the same order. */
export function ServiceFormBody({
  service,
  draft,
  test,
}: {
  service: Service;
  draft: ServiceDraft;
  test: ServiceTest;
}) {
  const status = cardStatus(configured(service), test.result);

  /* Ask the service what it offers using credentials it has already answered
     on: the ones a test just accepted, or the stored ones. Reading the draft
     here instead would call Radarr on every keystroke, and would lose the lists
     the moment an unrelated field is edited. */
  const options = useServiceOptions(
    service.kind,
    test.probed ?? (configured(service) ? service.config : null),
    service.id || undefined,
  );

  return (
    <>
      <p className="sg-modal-status">
        <span className="sg-dot" data-state={status.state} aria-hidden="true" />
        <span role="status">{test.pending ? 'Testing…' : status.label}</span>
      </p>

      <div className="sg-card-fields">
        <TextField
          id={`svc-${service.id}-name`}
          label="Connection name"
          value={draft.name}
          locked={service.locked}
          placeholder={`${kindLabel(service.kind)} - Default`}
          onChange={draft.setName}
        />

        <ServiceFields service={service} draft={draft} options={options.data} />

        <CheckField
          id={`svc-${service.id}-enabled`}
          label="Use this connection"
          checked={draft.enabled}
          locked={service.locked}
          onChange={draft.setEnabled}
        />
      </div>
    </>
  );
}

export function TestButton({ test }: { test: ServiceTest }) {
  const failed = test.result?.ok === false;
  return (
    <button
      type="button"
      className={`btn ${failed ? 'btn-primary' : 'btn-secondary'}`}
      disabled={test.pending}
      onClick={test.run}
    >
      <Unplug aria-hidden="true" size={16} />
      {test.pending ? 'Testing…' : failed ? 'Retry' : 'Test connection'}
    </button>
  );
}

/* The draft lives with the dialog, so closing it drops pending edits and the
   next open starts from the stored record. */
export function ServiceDialog({ service, onClose }: { service: Service; onClose: () => void }) {
  const draft = useServiceDraft(service);
  const save = useSaveService();
  const remove = useDeleteService();
  const test = useTestDraft(service.kind, draft, service.id);
  const [confirming, setConfirming] = useState(false);

  return (
    <>
      <Modal
        open
        onClose={onClose}
        title={draft.name}
        description={
          service.locked
            ? `A ${kindLabel(service.kind)} connection, pinned by an environment variable — edit it there.`
            : `A ${kindLabel(service.kind)} connection. Only you use it; other members keep their own.`
        }
        size="lg"
        footer={
          <>
            {/* Always live. Testing no longer saves, so a greyed-out Save after
                a test would read as a broken button. */}
            <button
              type="button"
              className="btn btn-primary"
              disabled={save.isPending}
              onClick={() => {
                if (!draft.dirty) {
                  onClose();
                  return;
                }
                save.mutate(
                  { id: service.id, patch: draft.patch },
                  {
                    onSuccess: () => {
                      draft.reset();
                      pushToast('Service saved');
                      onClose();
                    },
                  },
                )
              }}
            >
              <Save aria-hidden="true" size={16} />
              {save.isPending ? 'Saving…' : 'Save'}
            </button>
            <TestButton test={test} />
            <button
              type="button"
              className="btn btn-danger ml-auto"
              disabled={remove.isPending || service.locked}
              onClick={() => setConfirming(true)}
            >
              <Trash2 aria-hidden="true" size={16} />
              Delete
            </button>
          </>
        }
      >
        <ServiceFormBody service={service} draft={draft} test={test} />
      </Modal>

      <ConfirmDialog
        open={confirming}
        title={`Delete ${service.name}?`}
        body={`The ${kindLabel(service.kind)} connection is removed, along with everything Snagarr indexed through it.`}
        confirmLabel="Delete"
        pending={remove.isPending}
        onClose={() => setConfirming(false)}
        onConfirm={() =>
          remove.mutate(service.id, {
            onSuccess: () => {
              pushToast('Service deleted');
              setConfirming(false);
              onClose();
            },
          })
        }
      />
    </>
  );
}
