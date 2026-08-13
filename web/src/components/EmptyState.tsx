import { useStatus } from '../lib/queries';

/* The link is the operator's published iCloud Shortcut. /settings is admin only,
   so it rides on /status, which every role can read. No link, no button — a
   button that goes nowhere is worse than no button. */
export function EmptyState() {
  const status = useStatus();
  const shortcutUrl = status.data?.shortcut_url?.trim() ?? '';

  return (
    <section className="sg-empty sg-pad">
      <span className="sg-empty-poster sg-p" aria-hidden="true">Your<br />first<br />snag</span>
      <h3>Your list is ready for its first title</h3>
      <p className="text-muted max-w-[360px] text-[14px]">
        Search by name or paste a link above. Snagarr will identify it and keep track of what
        happens next.
      </p>
      {shortcutUrl !== '' && (
        <a className="btn btn-secondary" href={shortcutUrl} target="_blank" rel="noreferrer">
          Install the iOS Shortcut
        </a>
      )}
    </section>
  );
}
