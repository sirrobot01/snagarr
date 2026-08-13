import { useStatus } from '../lib/queries';

/* The link is the operator's published iCloud Shortcut. /settings is admin only,
   so it rides on /status, which every role can read. No link, no button — a
   button that goes nowhere is worse than no button. */
export function EmptyState() {
  const status = useStatus();
  const shortcutUrl = status.data?.shortcut_url?.trim() ?? '';

  return (
    <section className="sg-pad flex flex-col items-center py-8 text-center">
      <div className="sg-empty-poster" aria-hidden="true" />
      <h3 className="mt-4 text-[22px]">Nothing snagged yet.</h3>
      <p className="text-muted max-w-[280px] text-[13px]">
        Type a title above, or paste a link from anywhere. Snagarr resolves it and keeps the raw
        input either way.
      </p>
      {shortcutUrl !== '' && (
        <a className="btn btn-secondary" href={shortcutUrl} target="_blank" rel="noreferrer">
          Install the iOS Shortcut
        </a>
      )}
    </section>
  );
}
