import * as Dialog from '@radix-ui/react-dialog';
import { ItemDetail } from './ItemDetail';
import type { Item } from '../lib/types';

interface Props {
  item: Item | null;
  admin: boolean;
  onClose: () => void;
}

export function DetailSheet({ item, admin, onClose }: Props) {
  return (
    <Dialog.Root open={item !== null} onOpenChange={(open) => !open && onClose()}>
      <Dialog.Portal>
        <Dialog.Overlay className="sg-scrim" />
        <Dialog.Content className="sg-sheet" aria-describedby={undefined}>
          {item && (
            <>
              <Dialog.Title className="sr-only">{item.title}</Dialog.Title>
              <ItemDetail item={item} admin={admin} variant="sheet" onDone={onClose} />
            </>
          )}
        </Dialog.Content>
      </Dialog.Portal>
    </Dialog.Root>
  );
}
