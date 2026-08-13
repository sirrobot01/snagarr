import * as Popover from '@radix-ui/react-popover';
import { useState, type ReactNode } from 'react';
import { ItemDetail } from './ItemDetail';
import type { Item } from '../lib/types';

interface Props {
  item: Item;
  admin: boolean;
  children: ReactNode;
}

export function DetailPopover({ item, admin, children }: Props) {
  const [open, setOpen] = useState(false);

  return (
    <Popover.Root open={open} onOpenChange={setOpen}>
      <Popover.Trigger asChild>{children}</Popover.Trigger>
      <Popover.Portal>
        <Popover.Content className="sg-pop elev-lg" sideOffset={4} collisionPadding={16}>
          <ItemDetail item={item} admin={admin} variant="popover" onDone={() => setOpen(false)} />
        </Popover.Content>
      </Popover.Portal>
    </Popover.Root>
  );
}
