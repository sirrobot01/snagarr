import type { LucideIcon } from 'lucide-react';
import type { CardStatus } from './service';

interface Props {
  name: string;
  configured: boolean;
  state: CardStatus['state'];
  label: string;
  icon?: LucideIcon;
  note?: string;
}

/** The head of the two install-wide cards, TMDB and General. */
export function CardHead({ name, configured: ready, state, label, icon: Icon, note }: Props) {
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
