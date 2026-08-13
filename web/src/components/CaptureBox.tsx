import { forwardRef, type KeyboardEvent } from 'react';

interface Props {
  id: string;
  value: string;
  onChange: (value: string) => void;
  onKeyDown: (event: KeyboardEvent<HTMLInputElement>) => void;
  focused: boolean;
  onFocusChange: (focused: boolean) => void;
}

export const CaptureBox = forwardRef<HTMLInputElement, Props>(function CaptureBox(
  { id, value, onChange, onKeyDown, focused, onFocusChange },
  ref,
) {
  const live = focused || value.length > 0;

  return (
    <section className="sg-region sg-pad pb-4 pt-4 md:pb-[18px] md:pt-[26px]">
      <div className="flex items-center gap-3">
        <span className="sg-b sg-new sg-hint" aria-hidden="true">
          /
        </span>

        <input
          id={id}
          ref={ref}
          className="sg-search flex-1"
          type="search"
          autoFocus
          autoComplete="off"
          autoCorrect="off"
          spellCheck={false}
          enterKeyHint="go"
          aria-label="Snag a title or paste a link"
          placeholder="Snag anything…"
          value={value}
          onChange={(event) => onChange(event.target.value)}
          onKeyDown={onKeyDown}
          onFocus={() => onFocusChange(true)}
          onBlur={() => onFocusChange(false)}
        />

        <span className="sg-k hidden text-right md:block">
          LOCAL INDEX
          <br />+ TMDB
        </span>
      </div>

      <div className="sg-rule mt-3 md:hidden" data-idle={live ? undefined : '1'} />
    </section>
  );
});
