import { useState } from 'react';

type Theme = 'dark' | 'light';

function current(): Theme {
  return document.documentElement.dataset.theme === 'light' ? 'light' : 'dark';
}

export function ThemeToggle() {
  const [theme, setTheme] = useState<Theme>(current);

  function flip() {
    const next: Theme = theme === 'dark' ? 'light' : 'dark';
    document.documentElement.dataset.theme = next;
    localStorage.setItem('snagarr.theme', next);
    setTheme(next);
  }

  return (
    <button
      type="button"
      className="sg-theme-toggle"
      onClick={flip}
      aria-label={`Switch to ${theme === 'dark' ? 'light' : 'dark'} theme`}
    >
      {theme === 'dark' ? 'LIGHT' : 'DARK'}
    </button>
  );
}
