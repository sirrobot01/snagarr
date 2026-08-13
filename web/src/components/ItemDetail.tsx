import { Info } from 'lucide-react';
import { Badge } from './Badge';
import { ItemActions } from './ItemActions';
import { Poster } from './Poster';
import { captureContext, itemMeta } from '../lib/format';
import type { Item } from '../lib/types';

interface Props {
  item: Item;
  admin: boolean;
  variant: 'sheet' | 'popover';
  onDone: () => void;
}

export function ItemDetail({ item, admin, variant, onDone }: Props) {
  const sheet = variant === 'sheet';
  const size = sheet ? { width: 74, height: 111 } : { width: 80, height: 120 };

  return (
    <>
      <div className={sheet ? 'p-4' : 'p-4 pb-3'}>
        <div className="flex gap-4">
          <Poster path={item.poster_path} title={item.title} size="w342" {...size} />

          <div className="min-w-0 flex-1">
            <h3 className={sheet ? 'text-[24px]' : 'text-[19px]'}>
              {item.title}
            </h3>
            <p className="text-muted text-[12px]">{itemMeta(item)}</p>
            <Badge state={item.status} />
          </div>
        </div>

        {item.overview && (
          <p className={`mt-4 leading-[1.5] ${sheet ? 'text-[13px]' : 'text-[12px]'}`}>
            {item.overview}
          </p>
        )}

        {sheet && <hr className="hr" />}
        <p className={`sg-k flex items-center gap-2 ${sheet ? '' : 'mt-3'}`}>
          <Info aria-hidden="true" size={14} />
          {captureContext(item)}
        </p>
      </div>

      <ItemActions item={item} admin={admin} variant={variant} onDone={onDone} />
    </>
  );
}
