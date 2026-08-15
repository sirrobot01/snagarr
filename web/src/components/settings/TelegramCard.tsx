import { PlugZap, Send } from 'lucide-react';
import { useSaveSettings } from '../../lib/queries';
import type { CardProps } from './draft';
import { TextField } from './fields';
import { CardHead } from './CardHead';
import { cardStatus, useTelegramTest } from './service';

/* The household bot. One token serves the install; who may message it is the
   Telegram IDs on the household table, so this card only holds the token. */
export function TelegramCard({ settings, draft }: CardProps) {
  const current = { ...settings.telegram, ...draft.patch.telegram };
  const test = useTelegramTest();
  const save = useSaveSettings();
  const status = cardStatus(current.configured, test.result);
  const failed = test.result?.ok === false;
  const pending = test.pending || save.isPending;

  async function saveThenTest() {
    if (draft.dirty) {
      try {
        await save.mutateAsync(draft.patch);
      } catch {
        return;
      }
      draft.reset();
    }
    test.run();
  }

  return (
    <section className="sg-card">
      <CardHead
        name="Telegram bot"
        configured={current.configured}
        state={status.state}
        label={pending ? 'Testing…' : status.label}
        icon={Send}
        note="Members snag by messaging the bot — no port forwarding needed"
      />

      <TextField
        id="telegram-token"
        label="Bot token"
        value={current.bot_token}
        locked={current.locked}
        type="password"
        autoComplete="off"
        placeholder="123456789:AAF…"
        description="Create a bot with @BotFather and paste its token. Then put each member's Telegram ID on the household table below — the bot answers nobody else."
        onChange={(value) => draft.set('telegram', { bot_token: value })}
      />

      <button
        type="button"
        className={`btn ${failed ? 'btn-primary' : 'btn-secondary'} min-h-[44px] self-start`}
        style={{ fontSize: 12 }}
        disabled={pending}
        onClick={() => void saveThenTest()}
      >
        <PlugZap aria-hidden="true" size={16} />
        {failed ? 'Retry' : 'Test connection'}
      </button>
    </section>
  );
}
