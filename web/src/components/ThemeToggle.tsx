import { useState } from 'react';
import { Moon, Sun } from 'lucide-react';

type Theme = 'dark' | 'light';

function current(): Theme {
  return document.documentElement.dataset.theme === 'light' ? 'light' : 'dark';
}

export function ThemeToggle() {
  const [theme, setTheme] = useState<Theme>(current);
  const next = theme === 'dark' ? 'light' : 'dark';

  function flip() {
    document.documentElement.dataset.theme = next;
    localStorage.setItem('snagarr.theme', next);
    document
      .querySelector('meta[name="theme-color"]')
      ?.setAttribute('content', next === 'dark' ? '#201e1d' : '#f3f2f2');
    setTheme(next);
  }

  return (
    <button
      type="button"
      className="sg-theme-toggle"
      onClick={flip}
      aria-label={`Switch to ${next} theme`}
      title={`Switch to ${next} theme`}
    >
      {next === 'light' ? <Sun aria-hidden="true" /> : <Moon aria-hidden="true" />}
    </button>
  );
}
