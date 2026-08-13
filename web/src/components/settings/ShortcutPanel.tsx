import type { ShortcutLink } from '../../lib/types';
import { CopyField } from './fields';
import { QrCode } from './QrCode';

/* The phone fetches this link itself, so a LAN-only host fails on cellular and
   Shortcuts refuses a plain-http import. The server already knows whether the
   address it built is reachable; the public URL is the thing to go and fix. */
function warning(link: ShortcutLink, publicUrl: string): string | null {
  if (publicUrl.trim() === '') {
    return 'GENERAL HAS NO PUBLIC URL, SO THIS LINK POINTS AT THIS MACHINE. A PHONE CANNOT REACH IT — SET ONE FIRST.';
  }
  if (!link.public) {
    return 'THIS LINK IS NOT REACHABLE FROM OUTSIDE THE NETWORK. THE PHONE WILL FAIL TO DOWNLOAD IT.';
  }
  if (!/^https:\/\//i.test(link.url)) {
    return 'THE PUBLIC URL IS NOT HTTPS — SHORTCUTS WILL REFUSE THE IMPORT ON A PHONE.';
  }
  return null;
}

interface Props {
  name: string;
  link: ShortcutLink;
  publicUrl: string;
}

export function ShortcutPanel({ name, link, publicUrl }: Props) {
  const warn = warning(link, publicUrl);
  const expires = new Date(link.expires_at);

  return (
    <div className="flex flex-col gap-3 border-t border-line pt-4">
      <p className="sg-k m-0">APPLE SHORTCUT FOR {name.toUpperCase()}</p>
      {warn && <p className="sg-note m-0">{warn}</p>}

      <CopyField id="shortcut-url" label="Download link" value={link.url} />

      <div className="flex flex-wrap items-start gap-4">
        <QrCode text={link.import_url} label={`Import the Snag shortcut for ${name}`} />
        <p className="text-muted m-0 max-w-[40ch] text-[13px] leading-[1.5]">
          Scan this with the iPhone camera to import the Shortcut. The link carries a token, works
          once, and stops working
          {Number.isFinite(expires.getTime()) ? ` at ${expires.toLocaleTimeString()}` : ' shortly'}.
        </p>
      </div>
    </div>
  );
}
