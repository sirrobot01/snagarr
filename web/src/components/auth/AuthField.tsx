import type { InputHTMLAttributes, ReactNode } from 'react';

interface AuthFieldProps extends Omit<InputHTMLAttributes<HTMLInputElement>, 'className'> {
  label: string;
  error?: string;
  hint?: string;
  action?: ReactNode;
}

export function AuthField({
  label,
  error,
  hint,
  action,
  id,
  ...inputProps
}: AuthFieldProps) {
  const descriptionId = error ? `${id}-error` : hint ? `${id}-hint` : undefined;

  return (
    <div className="sg-auth-field">
      <label htmlFor={id}>{label}</label>
      <div className="sg-auth-input-wrap">
        <input
          {...inputProps}
          id={id}
          className="sg-auth-input"
          aria-invalid={Boolean(error) || undefined}
          aria-describedby={descriptionId}
        />
        {action}
      </div>
      {error ? (
        <p className="sg-auth-field-error" id={`${id}-error`}>
          {error}
        </p>
      ) : hint ? (
        <p className="sg-auth-field-hint" id={`${id}-hint`}>
          {hint}
        </p>
      ) : null}
    </div>
  );
}
