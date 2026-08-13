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

  // No sg-region: the rule under the field is this section's divider, and a
  // second full-width border below it reads as a mistake.
  return (
    <section className="sg-pad pb-4 pt-4 md:pb-[18px] md:pt-[26px]">
      <div className="flex items-center gap-3">
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
          placeholder="Search a title or paste a link"
          value={value}
          onChange={(event) => onChange(event.target.value)}
          onKeyDown={onKeyDown}
          onFocus={() => onFocusChange(true)}
          onBlur={() => onFocusChange(false)}
        />

        <span className="sg-search-source hidden items-center gap-2 md:flex">
          <span>
            Local library
            <br />
            + TMDB
          </span>
        </span>
      </div>

      <div className="sg-rule mt-3" data-idle={live ? undefined : '1'} />
    </section>
  );
});
