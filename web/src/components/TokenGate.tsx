import { useState } from 'react';
import { setToken } from '../lib/auth';
import { ThemeToggle } from './ThemeToggle';

export function TokenGate({ rejected }: { rejected: boolean }) {
  const [value, setValue] = useState('');

  return (
    <div className="sg-shell">
      <div className="flex flex-1 items-center justify-center p-4">
        <form
          className="w-full max-w-[380px]"
          onSubmit={(event) => {
            event.preventDefault();
            if (value.trim()) setToken(value);
          }}
        >
          <span className="sg-k">SNAGARR</span>
          <h2 className="mt-2">Enter your token</h2>
          <p className="text-muted max-w-[280px] text-[13px]">
            {rejected
              ? 'That token was rejected. Paste a current one, or open the setup URL the server printed.'
              : 'Paste the token the server printed on first run, or open its setup URL.'}
          </p>

          <div className="field mt-4">
            <label htmlFor="token">Token</label>
            <input
              id="token"
              className="input"
              type="password"
              autoComplete="off"
              autoFocus
              placeholder="sngr_…"
              aria-invalid={rejected || undefined}
              value={value}
              onChange={(event) => setValue(event.target.value)}
            />
          </div>

          <button type="submit" className="btn btn-primary btn-block" disabled={!value.trim()}>
            CONTINUE
          </button>
        </form>
      </div>
      <div className="sg-foot">
        <span className="sg-k">POWERED BY TMDB</span>
        <ThemeToggle />
      </div>
    </div>
  );
}
