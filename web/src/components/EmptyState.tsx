const SHORTCUT_URL = 'https://www.icloud.com/shortcuts/';

export function EmptyState() {
  return (
    <section className="sg-pad flex flex-col items-center py-8 text-center">
      <div className="sg-empty-poster" aria-hidden="true" />
      <h3 className="mt-4 text-[22px]">Nothing snagged yet.</h3>
      <p className="text-muted max-w-[280px] text-[13px]">
        Type a title above, or paste a link from anywhere. Snagarr resolves it and keeps the raw
        input either way.
      </p>
      <a className="btn btn-secondary" href={SHORTCUT_URL} target="_blank" rel="noreferrer">
        Install the iOS Shortcut
      </a>
    </section>
  );
}
