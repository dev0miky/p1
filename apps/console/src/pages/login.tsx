import { useState, type FormEvent } from "react";
import { motion } from "motion/react";
import { useAuth } from "@/lib/auth";
import { ApiError } from "@/lib/api";
import { Brand } from "@/components/brand";

export function Login() {
  const login = useAuth((s) => s.login);
  const [tenant, setTenant] = useState("");
  const [email, setEmail] = useState("");
  const [password, setPassword] = useState("");
  const [error, setError] = useState<string | null>(null);
  const [submitting, setSubmitting] = useState(false);

  async function onSubmit(e: FormEvent) {
    e.preventDefault();
    setError(null);
    setSubmitting(true);
    try {
      await login({ email, password, tenant_slug: tenant || undefined });
    } catch (err) {
      const msg = err instanceof ApiError
        ? (typeof err.body === "object" && err.body && "error" in err.body
            ? String((err.body as { error: unknown }).error)
            : `error ${err.status}`)
        : "network error";
      setError(msg);
      setSubmitting(false);
    }
  }

  return (
    <div className="relative min-h-full grid grid-cols-1 lg:grid-cols-[1.05fr_0.95fr]">
      <aside className="hidden lg:flex relative bg-ink-50 border-r border-ink-400 overflow-hidden">
        <div className="absolute inset-0 bg-scanlines opacity-[0.6] pointer-events-none" aria-hidden />
        <div className="absolute inset-0 bg-noise mix-blend-screen opacity-[0.5] pointer-events-none" aria-hidden />
        <div className="absolute top-10 left-10">
          <Brand size="md" />
        </div>
        <div className="relative m-auto w-full max-w-md px-12">
          <motion.div
            initial={{ opacity: 0, y: 14 }}
            animate={{ opacity: 1, y: 0 }}
            transition={{ duration: 0.7, ease: [0.16, 1, 0.3, 1] }}
          >
            <p className="font-mono text-2xs uppercase tracking-widest text-phosphor mb-6">
              outbound calling platform
            </p>
            <h1 className="font-display font-light text-[2.75rem] leading-[1.05] text-ink-950 tracking-tight">
              Press-1.<br />
              Broadcast.<br />
              <span className="text-ink-700">Run them right.</span>
            </h1>
            <div className="mt-12 space-y-3 max-w-sm">
              <Spec label="TCPA" value="audit-logged, append-only" />
              <Spec label="RLS" value="tenant-isolated by postgres" />
              <Spec label="A-attestation" value="STIR/SHAKEN on every leg" />
            </div>
          </motion.div>
        </div>
        <div className="absolute bottom-8 left-10 right-10 flex items-baseline justify-between font-mono text-2xs uppercase tracking-widest text-ink-700">
          <span>v 0.1.0 · dev</span>
          <span className="text-phosphor">● ops</span>
        </div>
      </aside>

      <main className="relative flex items-center justify-center px-6 py-16">
        <div className="absolute inset-0 bg-scanlines opacity-30 pointer-events-none" aria-hidden />
        <div className="lg:hidden absolute top-6 left-6">
          <Brand size="sm" />
        </div>
        <motion.form
          onSubmit={onSubmit}
          initial={{ opacity: 0, y: 12 }}
          animate={{ opacity: 1, y: 0 }}
          transition={{ duration: 0.5, ease: [0.16, 1, 0.3, 1], delay: 0.15 }}
          className="relative w-full max-w-[380px]"
        >
          <p className="font-mono text-2xs uppercase tracking-widest text-ink-700">step 01</p>
          <h2 className="mt-2 font-display font-light text-3xl text-ink-950 tracking-tight">Sign in</h2>
          <p className="mt-2 text-sm text-ink-800">
            Tenant accounts and the super-admin both land here.
          </p>

          <div className="mt-10 space-y-6">
            <Field
              label="Tenant slug"
              hint="leave blank for super-admin"
              value={tenant}
              onChange={setTenant}
              autoComplete="organization"
              mono
              placeholder="acme"
            />
            <Field
              label="Email"
              type="email"
              required
              value={email}
              onChange={setEmail}
              autoComplete="email"
              placeholder="you@example.com"
            />
            <Field
              label="Password"
              type="password"
              required
              value={password}
              onChange={setPassword}
              autoComplete="current-password"
              placeholder="•••••••••••"
            />
          </div>

          {error && (
            <div className="mt-6 border border-danger/30 bg-danger/[0.06] px-4 py-3 text-sm text-danger font-mono">
              {error}
            </div>
          )}

          <button
            type="submit"
            disabled={submitting}
            className="mt-8 w-full h-12 bg-phosphor text-ink-0 font-mono text-sm uppercase tracking-widest hover:bg-phosphor-glow disabled:opacity-50 disabled:cursor-not-allowed transition-colors flex items-center justify-center gap-3"
          >
            {submitting ? (
              <span className="flex items-center gap-2">
                <span className="status-dot bg-ink-0 animate-pulse-dot" />
                authenticating
              </span>
            ) : (
              <>
                enter <span className="opacity-50">→</span>
              </>
            )}
          </button>

          <p className="mt-10 font-mono text-2xs uppercase tracking-widest text-ink-700">
            no account? talk to your tenant owner.
          </p>
        </motion.form>
      </main>
    </div>
  );
}

function Spec({ label, value }: { label: string; value: string }) {
  return (
    <div className="grid grid-cols-[6rem_1fr] gap-4 items-baseline border-t border-ink-400 pt-3">
      <span className="font-mono text-2xs uppercase tracking-widest text-ink-700">{label}</span>
      <span className="font-mono text-xs text-ink-900">{value}</span>
    </div>
  );
}

interface FieldProps {
  label: string;
  value: string;
  onChange: (v: string) => void;
  type?: string;
  required?: boolean;
  placeholder?: string;
  autoComplete?: string;
  hint?: string;
  mono?: boolean;
}

function Field({ label, value, onChange, type = "text", required, placeholder, autoComplete, hint, mono }: FieldProps) {
  return (
    <label className="block group">
      <div className="flex items-baseline justify-between">
        <span className="field-label">{label}</span>
        {hint && <span className="font-mono text-2xs text-ink-700 normal-case tracking-normal">{hint}</span>}
      </div>
      <input
        type={type}
        required={required}
        placeholder={placeholder}
        autoComplete={autoComplete}
        value={value}
        onChange={(e) => onChange(e.target.value)}
        className={`mt-2 w-full h-11 bg-transparent border-b border-ink-400 px-0 text-ink-950 placeholder:text-ink-600 focus:border-phosphor transition-colors ${mono ? "font-mono" : ""}`}
      />
    </label>
  );
}
