import { Archive, Send } from 'lucide-react';

interface Props {
  count: number;
  admin: boolean;
  /** False when the household connects neither Radarr nor Sonarr. */
  canSend: boolean;
  onArchive: () => void;
  onSend: () => void;
}

export function SelectionBar({ count, admin, canSend, onArchive, onSend }: Props) {
  return (
    <div className="sg-selbar">
      <span className="sg-k">{count} selected</span>
      <button type="button" className="btn btn-secondary ml-auto min-h-[44px]" onClick={onArchive}>
        <Archive aria-hidden="true" size={16} />
        Archive
      </button>
      {admin && (
        <button
          type="button"
          className="btn btn-primary min-h-[44px]"
          disabled={!canSend}
          title={canSend ? undefined : 'Connect Radarr or Sonarr first'}
          onClick={onSend}
        >
          <Send aria-hidden="true" size={16} />
          Send to *Arr
        </button>
      )}
    </div>
  );
}
