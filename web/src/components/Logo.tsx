/* The mark is a notched square: a 96-unit box, a 24-unit stem and foot, no
   radius. Below 24px the notch fills in, so the brand set carries a heavier cut
   for small sizes and this picks between them. */
const FULL = 'M60 12 L84 12 L84 84 L12 84 L12 60 L60 60 Z';
const HEAVY = 'M52 6 L90 6 L90 90 L6 90 L6 52 L52 52 Z';

/** Set `title` where the mark stands alone. Beside the wordmark it is
    decoration, and a second label would only repeat what the text says. */
export function Logo({ size = 20, title }: { size?: number; title?: string }) {
  return (
    <svg
      className="sg-mark"
      viewBox="0 0 96 96"
      width={size}
      height={size}
      focusable="false"
      role={title ? 'img' : undefined}
      aria-label={title}
      aria-hidden={title ? undefined : true}
    >
      <path d={size < 24 ? HEAVY : FULL} fill="currentColor" />
    </svg>
  );
}
