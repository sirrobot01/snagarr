import type { ReactNode } from 'react';
import { RotateCcw, Save } from 'lucide-react';
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
      <Head
        eyebrow="Personal"
        title="My connections"
        note="Connect the services you use. Other household members manage their own."
      />
      <MyServices />

      {admin && settings.isError && (
        <div className="sg-pad">
          <ErrorState error={settings.error} onRetry={() => void settings.refetch()} />
        </div>
      )}
      {admin && !settings.isError && !settings.data && (
        <div className="sg-pad">
          <Loading label="Loading settings…" />
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
      <Head
        eyebrow="Administrator"
        title="Household settings"
        note="These settings affect everyone who uses this Snagarr installation."
      />
      <div className="sg-cards">
        <GeneralCard settings={settings} draft={draft} />
        <TmdbCard settings={settings} draft={draft} />
      </div>

      {draft.dirty && (
        <div className="sg-pad sg-region flex flex-wrap items-center gap-3 py-3">
          <span className="sg-k">Unsaved changes</span>
          <button
            type="button"
            className="btn btn-primary ml-auto min-h-[44px]"
            disabled={save.isPending}
            onClick={() =>
              save.mutate(draft.patch, {
                onSuccess: () => {
                  draft.reset();
                  pushToast('Settings saved');
                },
              })
            }
          >
            <Save aria-hidden="true" size={16} />
            {save.isPending ? 'Saving…' : 'Save changes'}
          </button>
          <button type="button" className="btn btn-ghost min-h-[44px]" onClick={draft.reset}>
            <RotateCcw aria-hidden="true" size={15} />
            Discard
          </button>
        </div>
      )}

      <HouseholdSection publicUrl={settings.general.public_url} meId={meId} />
    </>
  );
}

function Head({
  eyebrow,
  title,
  note,
}: {
  eyebrow: string;
  title: string;
  note: string;
}) {
  return (
    <div className="sg-page-head sg-pad">
      <div>
        <p className="sg-k m-0">{eyebrow}</p>
        <h2 className="m-0">{title}</h2>
        <p className="text-muted m-0 text-[13px]">{note}</p>
      </div>
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
