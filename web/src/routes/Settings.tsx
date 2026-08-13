import type { ReactNode } from 'react';
import { useSettingsDraft } from '../components/settings/draft';
import { HouseholdSection } from '../components/settings/HouseholdSection';
import { IntegrationGrid } from '../components/settings/IntegrationGrid';
import { ErrorState, Loading } from '../components/settings/states';
import { isAdmin, useMe, useSaveSettings, useSettings } from '../lib/queries';
import { pushToast } from '../lib/toast';

export default function Settings() {
  const me = useMe();
  const admin = isAdmin(me.data);
  const settings = useSettings(admin);
  const draft = useSettingsDraft();
  const save = useSaveSettings();

  if (me.isError) {
    return (
      <Frame>
        <ErrorState error={me.error} onRetry={() => void me.refetch()} />
      </Frame>
    );
  }
  if (!me.data) {
    return (
      <Frame>
        <Loading />
      </Frame>
    );
  }
  if (!admin) {
    return (
      <Frame>
        <p className="sg-k m-0 py-6">SETTINGS ARE ADMIN ONLY — ASK AN ADMIN FOR ACCESS.</p>
      </Frame>
    );
  }
  if (settings.isError) {
    return (
      <Frame>
        <ErrorState error={settings.error} onRetry={() => void settings.refetch()} />
      </Frame>
    );
  }
  if (!settings.data) {
    return (
      <Frame>
        <Loading label="LOADING SETTINGS…" />
      </Frame>
    );
  }

  return (
    <>
      <div className="sg-pad pb-4 pt-6">
        <h2 className="m-0">Settings</h2>
      </div>

      <IntegrationGrid settings={settings.data} draft={draft} />

      {draft.dirty && (
        <div className="sg-pad sg-region flex flex-wrap items-center gap-3 py-3">
          <span className="sg-k">UNSAVED CHANGES</span>
          <button
            type="button"
            className="btn btn-primary ml-auto min-h-[44px]"
            disabled={save.isPending}
            onClick={() =>
              save.mutate(draft.patch, {
                onSuccess: () => {
                  draft.reset();
                  pushToast('SETTINGS SAVED');
                },
              })
            }
          >
            {save.isPending ? 'SAVING…' : 'Save changes'}
          </button>
          <button type="button" className="btn btn-ghost min-h-[44px]" onClick={draft.reset}>
            Discard
          </button>
        </div>
      )}

      <HouseholdSection publicUrl={settings.data.general.public_url} meId={me.data.id} />
    </>
  );
}

function Frame({ children }: { children: ReactNode }) {
  return (
    <div className="sg-pad pt-6">
      <h2 className="m-0">Settings</h2>
      {children}
    </div>
  );
}
