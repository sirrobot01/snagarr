import { useArchive, useDeleteItem, useSend } from '../lib/queries';
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

  const target = item.media_type === 'tv' ? 'sonarr' : 'radarr';
  const stacked = variant === 'sheet';
  const cls = stacked ? 'btn btn-block p-3' : 'btn';

  function run(fn: () => void) {
    fn();
    onDone();
  }

  return (
    <div className={stacked ? 'flex flex-col p-4 pt-0' : 'sg-pop-actions'}>
      {admin && (
        <button
          type="button"
          className={`${cls} btn-primary`}
          onClick={() => run(() => send.mutate({ item, value: target }))}
        >
          SEND TO {target.toUpperCase()}
        </button>
      )}

      {admin && (
        <button
          type="button"
          className={`${cls} btn-secondary`}
          onClick={() => run(() => send.mutate({ item, value: 'overseerr' }))}
        >
          {stacked ? 'Request via Overseerr' : 'REQUEST'}
        </button>
      )}

      <button
        type="button"
        className={`${cls} btn-secondary`}
        onClick={() => run(() => archive.mutate({ item, value: !item.archived }))}
      >
        {item.archived ? 'Unarchive' : 'Archive'}
      </button>

      {admin && (
        <button
          type="button"
          className={`${cls} btn-danger`}
          onClick={() => run(() => remove.mutate({ item, value: undefined }))}
        >
          Delete
        </button>
      )}
    </div>
  );
}
