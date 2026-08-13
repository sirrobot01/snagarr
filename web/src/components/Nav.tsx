import { useEffect, useRef, useState } from 'react';
import { CircleUserRound, Library, Search, Settings2 } from 'lucide-react';
import { Link, useLocation } from 'wouter';

import { clearToken } from '../lib/auth';
import type { UserRef } from '../lib/types';
import { Logo } from './Logo';
import { ThemeToggle } from './ThemeToggle';

const LINKS = [
  { href: '/', label: 'Snag', icon: Search },
  { href: '/list', label: 'List', icon: Library },
  { href: '/settings', label: 'Settings', icon: Settings2 },
];

export function Nav({ me }: { me: UserRef | undefined }) {
  const [location] = useLocation();
  const [accountOpen, setAccountOpen] = useState(false);
  const accountRef = useRef<HTMLDivElement>(null);

  useEffect(() => setAccountOpen(false), [location]);

  useEffect(() => {
    if (!accountOpen) return;

    function onPointerDown(event: PointerEvent) {
      if (!accountRef.current?.contains(event.target as Node)) setAccountOpen(false);
    }
    function onKeyDown(event: KeyboardEvent) {
      if (event.key === 'Escape') setAccountOpen(false);
    }

    document.addEventListener('pointerdown', onPointerDown);
    document.addEventListener('keydown', onKeyDown);
    return () => {
      document.removeEventListener('pointerdown', onPointerDown);
      document.removeEventListener('keydown', onKeyDown);
    };
  }, [accountOpen]);

  return (
    <header className="nav">
      <div className="nav-inner">
        {/* The label goes on a narrow screen and the mark carries the brand,
            which is what the mark is for. */}
        <Link href="/" className="nav-brand" aria-label="Snagarr home">
          <Logo size={20} />
          <span className="nav-brand-label">Snagarr</span>
        </Link>

        <nav className="nav-links" aria-label="Main navigation">
          {LINKS.map(({ href, label, icon: Icon }) => (
            <Link
              key={href}
              href={href}
              className="nav-link"
              aria-label={label}
              aria-current={location === href ? 'page' : undefined}
            >
              <Icon className="nav-link-icon" aria-hidden="true" />
              <span className="nav-link-label">{label}</span>
            </Link>
          ))}
        </nav>

        <span className="nav-spacer" />
        <ThemeToggle />

        {me && (
          <div className="nav-account" ref={accountRef}>
            <button
              type="button"
              className="nav-account-trigger"
              aria-label={`Account menu for ${me.username}`}
              aria-haspopup="menu"
              aria-expanded={accountOpen}
              aria-controls="account-menu"
              onClick={() => setAccountOpen((open) => !open)}
            >
              <span className="nav-avatar" aria-hidden="true">
                <CircleUserRound size={19} />
              </span>
              <span className="nav-account-copy">
                <span className="nav-account-name">{me.username}</span>
              </span>
            </button>

            {accountOpen && (
              <div className="nav-menu" id="account-menu" role="menu">
                <div className="nav-menu-meta">
                  <strong>{me.username}</strong>
                  <span>Signed in · {me.role}</span>
                </div>
                <button
                  type="button"
                  className="nav-menu-item"
                  role="menuitem"
                  onClick={clearToken}
                >
                  Sign out
                </button>
              </div>
            )}
          </div>
        )}
      </div>
    </header>
  );
}
