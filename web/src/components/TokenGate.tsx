import { useCallback, useEffect, useState, type FormEvent } from 'react';
import { useLocation } from 'wouter';
import {
  AlertCircle,
  ArrowRight,
  Eye,
  EyeOff,
  LoaderCircle,
  LockKeyhole,
  Sparkles,
} from 'lucide-react';

import {
  AuthRequestError,
  getAuthStatus,
  registerAccount,
  setToken,
  signIn,
} from '../lib/auth';
import { ThemeToggle } from './ThemeToggle';
import { AuthField } from './auth/AuthField';
import { BrandMark, WelcomePanel } from './auth/WelcomePanel';

type AuthMode = 'register' | 'login';
type FieldName = 'username' | 'password' | 'confirmPassword';
type FieldErrors = Partial<Record<FieldName, string>>;

interface FormValues {
  username: string;
  password: string;
  confirmPassword: string;
}

const emptyForm: FormValues = {
  username: '',
  password: '',
  confirmPassword: '',
};

function validate(mode: AuthMode, values: FormValues): FieldErrors {
  const errors: FieldErrors = {};
  const username = values.username.trim();

  if (!username) errors.username = 'Enter your username.';
  else if (
    mode === 'register' &&
    !/^[a-zA-Z0-9][a-zA-Z0-9._-]{2,31}$/.test(username)
  ) {
    errors.username = 'Use 3–32 letters, numbers, dots, dashes, or underscores.';
  }

  if (!values.password) errors.password = 'Enter your password.';

  if (mode === 'register') {
    if (!values.confirmPassword) errors.confirmPassword = 'Confirm your password.';
    else if (values.password !== values.confirmPassword) {
      errors.confirmPassword = 'Those passwords do not match.';
    }
  }

  return errors;
}

function messageFor(error: unknown) {
  return error instanceof AuthRequestError
    ? error.message
    : 'Something unexpected happened. Please try again.';
}

export function TokenGate({ rejected = false }: { rejected?: boolean }) {
  const [, navigate] = useLocation();
  const [mode, setMode] = useState<AuthMode | null>(null);
  const [setupRequired, setSetupRequired] = useState(false);
  const [statusError, setStatusError] = useState('');
  const [values, setValues] = useState<FormValues>(emptyForm);
  const [fieldErrors, setFieldErrors] = useState<FieldErrors>({});
  const [formError, setFormError] = useState('');
  const [submitting, setSubmitting] = useState(false);
  const [showPassword, setShowPassword] = useState(false);

  const loadStatus = useCallback(async (signal?: AbortSignal) => {
    setStatusError('');
    setMode(null);
    try {
      const status = await getAuthStatus(signal);
      setMode(status.initialized ? 'login' : 'register');
      setSetupRequired(status.setup_required);
      if (rejected) setFormError('Your session ended. Sign in again to continue.');
    } catch (error) {
      if (error instanceof DOMException && error.name === 'AbortError') return;
      setStatusError(messageFor(error));
    }
  }, [rejected]);

  useEffect(() => {
    const controller = new AbortController();
    void loadStatus(controller.signal);
    return () => controller.abort();
  }, [loadStatus]);

  function update(field: FieldName, value: string) {
    setValues((current) => ({ ...current, [field]: value }));
    setFieldErrors((current) => {
      if (!current[field]) return current;
      const next = { ...current };
      delete next[field];
      return next;
    });
    if (formError) setFormError('');
  }

  async function submit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    if (!mode || submitting) return;

    const errors = validate(mode, values);
    setFieldErrors(errors);
    setFormError('');
    const firstError = Object.keys(errors)[0] as FieldName | undefined;
    if (firstError) {
      document.getElementById(`auth-${firstError}`)?.focus();
      return;
    }

    setSubmitting(true);
    try {
      const session =
        mode === 'register'
          ? await registerAccount({
              username: values.username.trim(),
              password: values.password,
            })
          : await signIn({ username: values.username.trim(), password: values.password });
      if (mode === 'register' || setupRequired) navigate('/setup', { replace: true });
      else navigate('/', { replace: true });
      setToken(session.token);
    } catch (error) {
      if (mode === 'register' && error instanceof AuthRequestError && error.status === 409) {
        setValues((current) => ({ ...emptyForm, username: current.username }));
        setMode('login');
        setFormError('Setup was completed in another tab. Sign in to continue.');
      } else {
        setFormError(messageFor(error));
      }
    } finally {
      setSubmitting(false);
    }
  }

  const passwordAction = (
    <button
      type="button"
      className="sg-auth-password-toggle"
      onClick={() => setShowPassword((visible) => !visible)}
      aria-label={showPassword ? 'Hide password' : 'Show password'}
      aria-pressed={showPassword}
    >
      {showPassword ? <EyeOff size={18} /> : <Eye size={18} />}
    </button>
  );

  return (
    <div className="sg-auth-shell">
      <WelcomePanel />

      <main className="sg-auth-form-side">
        <div className="sg-auth-mobile-head">
          <BrandMark compact />
          <ThemeToggle />
        </div>
        <div className="sg-auth-theme">
          <ThemeToggle />
        </div>

        <section className="sg-auth-card" aria-labelledby="auth-title">
          {statusError ? (
            <div className="sg-auth-status-error">
              <span className="sg-auth-status-icon" aria-hidden="true">
                <AlertCircle size={22} />
              </span>
              <p className="sg-auth-eyebrow">Connection problem</p>
              <h1 id="auth-title">We couldn’t reach Snagarr</h1>
              <p role="alert">{statusError}</p>
              <button className="sg-auth-submit" type="button" onClick={() => void loadStatus()}>
                Try again
                <ArrowRight size={18} aria-hidden="true" />
              </button>
            </div>
          ) : !mode ? (
            <div className="sg-auth-loading" role="status">
              <LoaderCircle size={24} className="sg-auth-spinner" aria-hidden="true" />
              <span>Preparing your account…</span>
            </div>
          ) : (
            <>
              <div className="sg-auth-card-head">
                <span className="sg-auth-status-icon" aria-hidden="true">
                  {mode === 'register' ? <Sparkles size={21} /> : <LockKeyhole size={21} />}
                </span>
                <p className="sg-auth-eyebrow">
                  {mode === 'register' ? 'First-time setup' : 'Welcome back'}
                </p>
                <h1 id="auth-title">
                  {mode === 'register' ? 'Create your account' : 'Sign in to Snagarr'}
                </h1>
                <p className="sg-auth-card-copy">
                  {mode === 'register'
                    ? 'Set up the administrator account for this Snagarr server.'
                    : 'Use the account credentials for this Snagarr server.'}
                </p>
              </div>

              <form className="sg-auth-form" onSubmit={submit} noValidate aria-busy={submitting}>
                {formError && (
                  <div className="sg-auth-error" role="alert">
                    <AlertCircle size={17} aria-hidden="true" />
                    <span>{formError}</span>
                  </div>
                )}

                <AuthField
                  id="auth-username"
                  label="Username"
                  type="text"
                  autoComplete="username"
                  autoCapitalize="none"
                  spellCheck={false}
                  autoFocus
                  placeholder="alex"
                  value={values.username}
                  error={fieldErrors.username}
                  hint={mode === 'register' ? 'You’ll use this name when you sign in.' : undefined}
                  onChange={(event) => update('username', event.target.value)}
                />

                <AuthField
                  id="auth-password"
                  label="Password"
                  action={passwordAction}
                  type={showPassword ? 'text' : 'password'}
                  autoComplete={mode === 'register' ? 'new-password' : 'current-password'}
                  placeholder="Enter your password"
                  value={values.password}
                  error={fieldErrors.password}
                  onChange={(event) => update('password', event.target.value)}
                />

                {mode === 'register' && (
                  <AuthField
                    id="auth-confirmPassword"
                    label="Confirm password"
                    type={showPassword ? 'text' : 'password'}
                    autoComplete="new-password"
                    placeholder="Type it once more"
                    value={values.confirmPassword}
                    error={fieldErrors.confirmPassword}
                    onChange={(event) => update('confirmPassword', event.target.value)}
                  />
                )}

                <button className="sg-auth-submit" type="submit" disabled={submitting}>
                  {submitting ? (
                    <>
                      <LoaderCircle size={18} className="sg-auth-spinner" aria-hidden="true" />
                      {mode === 'register' ? 'Creating account…' : 'Signing in…'}
                    </>
                  ) : (
                    <>
                      {mode === 'register' ? 'Create account' : 'Sign in'}
                      <ArrowRight size={18} aria-hidden="true" />
                    </>
                  )}
                </button>
              </form>

            </>
          )}
        </section>
      </main>
    </div>
  );
}
