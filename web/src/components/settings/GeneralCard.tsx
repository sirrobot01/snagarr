import { Globe2, SendHorizontal, Webhook } from 'lucide-react';
import { CardHead } from './CardHead';
import type { CardProps } from './draft';
import { CheckField, TextField } from './fields';

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
          <SendHorizontal aria-hidden="true" size={14} /> Capture
        </h6>
        <CheckField
          id="general-auto-send"
          label="Send to Radarr or Sonarr automatically"
          checked={current.auto_send}
          locked={current.locked}
          onChange={(checked) => draft.set('general', { auto_send: checked })}
        />
        <p className="sg-field-help">
          A snagged title nobody owns yet goes straight to the capturer’s own download manager.
          Snagarr never spends another member’s service, so somebody with none connected keeps the
          Send button. Turn this off to send everything by hand.
        </p>
      </div>

      <div className="sg-card-section">
        <h6 className="sg-card-section-title flex items-center gap-2">
          <Webhook aria-hidden="true" size={14} /> Webhooks
        </h6>
        <p className="sg-field-help break-all">
          Add this URL to Radarr: <code>{`${base}/api/v1/webhooks/radarr`}</code>. Change{' '}
          <code>radarr</code> to <code>sonarr</code>, <code>tautulli</code>, <code>emby</code>, or{' '}
          <code>jellyfin</code> for other senders.
        </p>
        <p className="sg-field-help">
          Authenticate as a household member. Radarr and Sonarr have Username and Password fields
          on a webhook connection — use a member’s sign-in details. A sender that sets its own
          headers can send <code>Authorization: Bearer &lt;token&gt;</code>. One that can do
          neither, such as Emby, can put <code>?token=&lt;token&gt;</code> on the URL.
        </p>
      </div>
    </section>
  );
}
