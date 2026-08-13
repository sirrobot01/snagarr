import type { CardProps } from './draft';
import { CopyField, TextField } from './fields';
import { CardHead } from './ServiceCard';

export function GeneralCard({ settings, draft }: CardProps) {
  const current = { ...settings.general, ...draft.patch.general };
  const base = (current.public_url.trim() === '' ? window.location.origin : current.public_url)
    .trim()
    .replace(/\/+$/, '');

  return (
    <section className="sg-card">
      <CardHead
        name="General"
        configured={current.configured}
        state={current.configured ? 'ok' : 'unset'}
        label={current.configured ? 'OK' : 'NOT SET'}
      />

      <TextField
        id="general-public-url"
        label="Public URL"
        value={current.public_url}
        locked={current.locked}
        inputMode="url"
        placeholder="http://localhost:8080"
        onChange={(value) => draft.set('general', { public_url: value })}
      />

      <TextField
        id="general-reconcile"
        label="Reconcile interval"
        value={current.reconcile_interval}
        locked={current.locked}
        placeholder="15m"
        onChange={(value) => draft.set('general', { reconcile_interval: value })}
      />

      <CopyField id="general-webhook" label="Webhook secret" value={current.webhook_secret} />
      <p className="text-muted m-0 break-all text-[11px] leading-[1.5]">
        Webhook URL: <code>{`${base}/api/v1/webhooks/radarr?secret=${current.webhook_secret}`}</code>
        . Replace <code>radarr</code> with <code>sonarr</code>, <code>tautulli</code> or{' '}
        <code>emby</code> for the other senders.
      </p>
    </section>
  );
}
