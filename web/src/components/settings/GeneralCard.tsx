import { Globe2, Link2, Webhook } from 'lucide-react';
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
        label={current.configured ? 'Ready' : 'Not set'}
        icon={Globe2}
        note="How Snagarr is reached and kept in sync"
      />

      <div className="sg-card-section">
        <h6 className="sg-card-section-title">Server</h6>
        <div className="sg-field-grid">
          <TextField
            id="general-public-url"
            label="Public URL"
            value={current.public_url}
            locked={current.locked}
            inputMode="url"
            type="url"
            placeholder="https://snagarr.example.com"
            description="The address household members use to open Snagarr."
            onChange={(value) => draft.set('general', { public_url: value })}
          />

          <TextField
            id="general-reconcile"
            label="Library refresh interval"
            value={current.reconcile_interval}
            locked={current.locked}
            placeholder="15m"
            description="How often Snagarr checks connected services for changes."
            onChange={(value) => draft.set('general', { reconcile_interval: value })}
          />
        </div>
      </div>

      <div className="sg-card-section">
        <h6 className="sg-card-section-title flex items-center gap-2">
          <Link2 aria-hidden="true" size={14} /> iOS Shortcut
        </h6>
        <TextField
          id="general-shortcut-url"
          label="Published Shortcut link"
          value={current.shortcut_url}
          locked={current.locked}
          inputMode="url"
          type="url"
          placeholder="https://www.icloud.com/shortcuts/…"
          description="In Apple Shortcuts, choose Share → Copy iCloud Link, then paste it here."
          onChange={(value) => draft.set('general', { shortcut_url: value })}
        />
      </div>

      <div className="sg-card-section">
        <h6 className="sg-card-section-title flex items-center gap-2">
          <Webhook aria-hidden="true" size={14} /> Webhooks
        </h6>
        <CopyField id="general-webhook" label="Webhook secret" value={current.webhook_secret} />
        <p className="sg-field-help break-all">
          Add this URL to Radarr:{' '}
          <code>{`${base}/api/v1/webhooks/radarr?secret=${current.webhook_secret}`}</code>. Change{' '}
          <code>radarr</code> to <code>sonarr</code>, <code>tautulli</code>, or <code>emby</code> for
          other senders.
        </p>
      </div>
    </section>
  );
}
