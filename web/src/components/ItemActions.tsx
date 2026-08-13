import { Archive, ArchiveRestore, Send, SendHorizontal, Trash2 } from 'lucide-react';
import { useArchive, useDeleteItem, useSend, useStatus } from '../lib/queries';
import type { Item } from '../lib/types';

interface Props {
  item: Item;
  admin: boolean;
  variant: 'sheet' | 'popover';
  onDone: () => void;
}

export function ItemActions({ item, admin, variant, onDone }: Props) {
  const send = useSend();
  const archive = useArchive();
  const remove = useDeleteItem();
  const status = useStatus();

  const target = item.media_type === 'tv' ? 'sonarr' : 'radarr';
  const stacked = variant === 'sheet';
  const cls = stacked ? 'btn btn-block p-3' : 'btn';

  // A service nobody connected cannot take the title, so the button says so
  // rather than failing after the click. A series needs Sonarr and a film
  // Radarr: having one is no help to the other.
  const services = status.data?.services;
  const arr = target === 'sonarr' ? 'Sonarr' : 'Radarr';
  const canSend = services?.[target] ?? false;
  const canRequest = services?.overseerr ?? false;

  function run(fn: () => void) {
    fn();
    onDone();
  }

  return (
    <div className={stacked ? 'sg-sheet-actions flex flex-col p-4 pt-0' : 'sg-pop-actions'}>
      {admin && (
        <button
          type="button"
          className={`${cls} btn-primary`}
          disabled={!canSend}
          title={canSend ? undefined : `${arr} is not connected`}
          onClick={() => run(() => send.mutate({ item, value: target }))}
        >
          <Send aria-hidden="true" size={16} />
          Send to {arr}
        </button>
      )}

      {admin && (
        <button
          type="button"
          className={`${cls} btn-secondary`}
          disabled={!canRequest}
          title={canRequest ? undefined : 'Overseerr is not connected'}
          onClick={() => run(() => send.mutate({ item, value: 'overseerr' }))}
        >
          <SendHorizontal aria-hidden="true" size={16} />
          {stacked ? 'Request via Overseerr' : 'Request'}
        </button>
      )}

      <button
        type="button"
        className={`${cls} btn-secondary`}
        onClick={() => run(() => archive.mutate({ item, value: !item.archived }))}
      >
        {item.archived ? (
          <ArchiveRestore aria-hidden="true" size={16} />
        ) : (
          <Archive aria-hidden="true" size={16} />
        )}
        {item.archived ? 'Unarchive' : 'Archive'}
      </button>

      {admin && (
        <button
          type="button"
          className={`${cls} btn-danger`}
          onClick={() => run(() => remove.mutate({ item, value: undefined }))}
        >
          <Trash2 aria-hidden="true" size={16} />
          Delete
        </button>
      )}
    </div>
  );
}
