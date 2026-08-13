import { Link, useLocation } from 'wouter';
import { ThemeToggle } from './ThemeToggle';
import type { UserRef } from '../lib/types';

const LINKS = [
  { href: '/', label: 'Snag' },
  { href: '/list', label: 'List' },
];

export function Nav({ me, admin }: { me: UserRef | undefined; admin: boolean }) {
  const [location] = useLocation();
  const links = admin ? [...LINKS, { href: '/settings', label: 'Settings' }] : LINKS;
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
