import type { ReactNode } from 'react';
import { useSettingsDraft, type Draft } from '../components/settings/draft';
import { GeneralCard } from '../components/settings/GeneralCard';
import { HouseholdSection } from '../components/settings/HouseholdSection';
import { MyServices } from '../components/settings/MyServices';
import { ErrorState, Loading } from '../components/settings/states';
import { TmdbCard } from '../components/settings/TmdbCard';
import { isAdmin, useMe, useSaveSettings, useSettings } from '../lib/queries';
import { pushToast } from '../lib/toast';
import type { Settings as GlobalSettings } from '../lib/types';

export default function Settings() {
  const me = useMe();
  const admin = isAdmin(me.data);
  /* Only an admin may read /settings, so a member never asks for it and the
     install-wide cards stay off their page entirely. */
  const settings = useSettings(admin);
  const draft = useSettingsDraft();

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

  return (
    <>
      <Head title="My services" note="Each member connects their own servers." />
      <MyServices />

      {admin && settings.isError && (
        <div className="sg-pad">
          <ErrorState error={settings.error} onRetry={() => void settings.refetch()} />
        </div>
      )}
      {admin && !settings.isError && !settings.data && (
        <div className="sg-pad">
          <Loading label="LOADING SETTINGS…" />
        </div>
      )}
      {admin && settings.data && (
        <GlobalCards settings={settings.data} draft={draft} meId={me.data.id} />
      )}
    </>
  );
}

function GlobalCards({
  settings,
  draft,
  meId,
}: {
  settings: GlobalSettings;
  draft: Draft;
  meId: number;
}) {
  const save = useSaveSettings();

  return (
    <>
      <Head title="Install settings" note="Shared by everybody in the household." />
      {/* Two cards, so the three-column track would leave a bare divider cell. */}
      <div className="sg-cards">
        <GeneralCard settings={settings} draft={draft} />
        <TmdbCard settings={settings} draft={draft} />
      </div>

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

      <HouseholdSection publicUrl={settings.general.public_url} meId={meId} />
    </>
  );
}

function Head({ title, note }: { title: string; note: string }) {
  return (
    <div className="sg-pad pb-3 pt-6">
      <h4 className="m-0">{title}</h4>
      <p className="text-muted m-0 text-[13px]">{note}</p>
    </div>
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
