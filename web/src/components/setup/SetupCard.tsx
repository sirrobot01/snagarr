import type { ReactNode } from 'react';

const SLOTS = [0, 1, 2, 3];

interface Props {
  step: number;
  title: string;
  copy: string;
  children: ReactNode;
  footer?: ReactNode;
}

export function SetupCard({ step, title, copy, children, footer }: Props) {
  return (
    <div className="sg-pad py-8">
      <div className="sg-setup-card">
        <div className="flex flex-col gap-2 px-6 pb-4 pt-6">
          <span className="sg-k">SETUP · STEP {step + 1} OF 4</span>
          <h2 className="m-0" style={{ fontSize: 34, letterSpacing: '-0.03em' }}>
            {title}
          </h2>
          <p className="m-0 text-[14px] text-muted">{copy}</p>
        </div>

        <div className="sg-progress">
          {SLOTS.map((slot) => (
            <span key={slot} data-on={slot <= step ? '1' : undefined} />
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
