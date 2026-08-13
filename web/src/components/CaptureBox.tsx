import { forwardRef, type KeyboardEvent } from 'react';
import { Search, X } from 'lucide-react';

import { useMediaQuery } from '../hooks/useMediaQuery';

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
  const touchPrimary = useMediaQuery('(pointer: coarse)');

  // No sg-region: the rule under the field is this section's divider, and a
  // second full-width border below it reads as a mistake.
  return (
    <section className="sg-capture sg-pad pb-4 pt-4 md:pb-[18px] md:pt-[26px]">
      <div className="sg-search-wrap flex items-center gap-3">
        <Search className="sg-capture-icon" aria-hidden="true" size={20} />
        <input
          id={id}
          ref={ref}
          className="sg-search flex-1"
          type="search"
          autoFocus={!touchPrimary}
          autoComplete="off"
          autoCorrect="off"
          spellCheck={false}
          enterKeyHint="search"
          aria-label="Snag a title or paste a link"
          placeholder="Search a title or paste a link"
          value={value}
          onChange={(event) => onChange(event.target.value)}
          onKeyDown={onKeyDown}
          onFocus={() => onFocusChange(true)}
          onBlur={() => onFocusChange(false)}
        />

        {value && (
          <button
            type="button"
            className="sg-search-clear"
            aria-label="Clear search"
            onClick={() => {
              onChange('');
              document.getElementById(id)?.focus();
            }}
          >
            <X aria-hidden="true" size={18} />
          </button>
        )}

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
