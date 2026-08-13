export function EmptyState() {
  return (
    <section className="sg-empty sg-pad">
      <span className="sg-empty-poster sg-p" aria-hidden="true">Your<br />first<br />snag</span>
      <h3>Your list is ready for its first title</h3>
      <p className="text-muted max-w-[360px] text-[14px]">
        Search by name or paste a link above. Snagarr will identify it and keep track of what
        happens next.
      </p>
    </section>
  );
}
