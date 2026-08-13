interface Props {
  count: number;
  admin: boolean;
  onArchive: () => void;
  onSend: () => void;
}

export function SelectionBar({ count, admin, onArchive, onSend }: Props) {
  return (
    <div className="sg-selbar">
      <span className="sg-k">{count} SELECTED</span>
      <button type="button" className="btn btn-secondary ml-auto min-h-[44px]" onClick={onArchive}>
        ARCHIVE
      </button>
      {admin && (
        <button type="button" className="btn btn-primary min-h-[44px]" onClick={onSend}>
          SEND TO RADARR
        </button>
      )}
    </div>
  );
}
