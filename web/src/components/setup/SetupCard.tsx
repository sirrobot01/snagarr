import type { ReactNode } from 'react';

interface Props {
  step: number;
  /** How many steps this wizard has; builds with the shared TMDB key skip one. */
  total: number;
  title: string;
  copy: string;
  children: ReactNode;
  footer?: ReactNode;
}

export function SetupCard({ step, total, title, copy, children, footer }: Props) {
  return (
    <div className="sg-pad py-8">
      <div className="sg-setup-card">
        <div className="flex flex-col gap-2 px-6 pb-4 pt-6">
          <span className="sg-k">
            Setup · Step {step + 1} of {total}
          </span>
          <h2 className="m-0" style={{ fontSize: 34, letterSpacing: '-0.03em' }}>
            {title}
          </h2>
          <p className="m-0 text-[14px] text-muted">{copy}</p>
        </div>

        <div className="sg-progress">
          {Array.from({ length: total }, (_, slot) => (
            <span key={slot} data-on={slot <= step ? '1' : undefined}>
              <span className="sr-only">Step {slot + 1}</span>
            </span>
          ))}
        </div>

        <div className="px-6 py-[22px]">{children}</div>

        {footer && (
          <div className="flex flex-wrap items-center gap-2 border-t-2 border-divider px-6 py-4">
            {footer}
          </div>
        )}
      </div>
    </div>
  );
}
