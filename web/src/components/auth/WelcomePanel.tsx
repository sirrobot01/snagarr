import { Logo } from '../Logo';

const features = [
  {
    index: '01',
    title: 'Capture in seconds',
    detail: 'Search once, then send the right title to the right service.',
  },
  {
    index: '02',
    title: 'Made for the household',
    detail: 'Everyone gets a personal queue without losing the shared view.',
  },
  {
    index: '03',
    title: 'One clean workflow',
    detail: 'Keep requests, reviews, and your library connected.',
  },
];

export function BrandMark({ compact = false }: { compact?: boolean }) {
  return (
    <div className="sg-auth-brand" data-compact={compact || undefined}>
      <Logo size={compact ? 22 : 26} />
      <span>Snagarr</span>
    </div>
  );
}

export function WelcomePanel() {
  return (
    <aside className="sg-auth-visual" aria-label="About Snagarr">
      <BrandMark />

      <div className="sg-auth-visual-copy">
        <p className="sg-auth-eyebrow">Your household watchlist</p>
        <h1>Find it. Snag it. Watch it.</h1>
        <p className="sg-auth-lede">
          A focused place for everyone at home to capture what they want to watch next.
        </p>

        <div className="sg-auth-features">
          {features.map(({ index, title, detail }) => (
            <div className="sg-auth-feature" key={title}>
              <span className="sg-auth-feature-index" aria-hidden="true">{index}</span>
              <div>
                <strong>{title}</strong>
                <span>{detail}</span>
              </div>
            </div>
          ))}
        </div>
      </div>
    </aside>
  );
}
