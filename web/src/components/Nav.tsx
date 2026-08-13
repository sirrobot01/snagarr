import { Link, useLocation } from 'wouter';
import { ThemeToggle } from './ThemeToggle';
import type { UserRef } from '../lib/types';

/* Settings is on every member's nav: it is where they connect their own Plex
   and their own Radarr. The install-wide cards inside are still admin-only. */
const LINKS = [
  { href: '/', label: 'Snag' },
  { href: '/list', label: 'List' },
  { href: '/settings', label: 'Settings' },
];

export function Nav({ me }: { me: UserRef | undefined }) {
  const [location] = useLocation();
  const links = LINKS;
  const other =
    location === '/' ? { href: '/list', label: 'LIST →' } : { href: '/', label: '← SNAG' };

  return (
    <header className="nav">
      <span className="nav-brand md:mr-0">SNAGARR</span>

      <nav className="hidden items-center gap-4 md:flex">
        {links.map((link) => (
          <Link
            key={link.href}
            href={link.href}
            aria-current={location === link.href ? 'page' : undefined}
          >
            {link.label}
          </Link>
        ))}
      </nav>

      {me && (
        <span className="sg-k ml-auto hidden md:inline">
          {me.display_name} · {me.role}
        </span>
      )}

      <Link href={other.href} className="sg-k ml-auto md:hidden">
        {other.label}
      </Link>

      <ThemeToggle />
    </header>
  );
}
