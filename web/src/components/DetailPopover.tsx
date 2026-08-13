import * as Popover from '@radix-ui/react-popover';
import { type ReactNode } from 'react';
import { ItemDetail } from './ItemDetail';
import type { Item } from '../lib/types';

interface Props {
  item: Item;
  admin: boolean;
  /** Held by the list, so a link that names an item can open it. */
  open: boolean;
  onOpenChange: (open: boolean) => void;
  children: ReactNode;
}

export function DetailPopover({ item, admin, open, onOpenChange, children }: Props) {
  return (
    <Popover.Root open={open} onOpenChange={onOpenChange}>
      <Popover.Trigger asChild>{children}</Popover.Trigger>
      <Popover.Portal>
        <Popover.Content className="sg-pop elev-lg" sideOffset={4} collisionPadding={16}>
          <ItemDetail
            item={item}
            admin={admin}
            variant="popover"
            onDone={() => onOpenChange(false)}
          />
        </Popover.Content>
      </Popover.Portal>
    </Popover.Root>
  );
}
