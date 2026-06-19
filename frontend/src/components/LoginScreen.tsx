import { useState, useEffect, useCallback } from 'react';
import { M3FadeIn, M3CircularProgress, M3ScaleIn, AnimatePresence } from './m3';
import { fetchMe, setup, login, onAuthLost } from '../lib/api';
import { AuthProvider, useAuth } from '../lib/auth-context';
import BrandMarkIcon from './BrandMarkIcon';

// LoginScreen acts as the auth gate for the entire application.
//
// It always renders an <AuthProvider> around its children, so any descendant
// (NavigationRail, UsersPanel, …) can read who is logged in via useAuth().
// The actual gate logic lives in <LoginGate>, which consumes the context.
//
// Flow on mount:
//   1. Call GET /api/me — exempt from auth middleware.
//   2. If response has loopback:true → render children (localhost, no auth needed).
//   3. If response has username → valid session → render children.
//   4. If 401 → call POST /api/setup to check if setup exists:
//      - 409 Conflict → setup already done → show login form.
//      - Otherwise → show setup form (first-run).
//
// The component also registers the 401 interceptor callback so that any
// mid-session 401 (e.g. expired session) drops back to the login form.

type AuthMode = 'setup' | 'login';

export default function LoginScreen({ children }: { children: React.ReactNode }) {
  // The provider must wrap children so that, once authenticated, <App/> and
  // its descendants can call useAuth(). We render it unconditionally here and
  // let <LoginGate> decide what to show (login card vs. children).
  return (
    <AuthProvider>
      <LoginGate>{children}</LoginGate>
    </AuthProvider>
  );
}

function LoginGate({ children }: { children: React.ReactNode }) {
  const { user, setUser } = useAuth();
  const [mode, setMode] = useState<AuthMode | null>(null);
  const [error, setError] = useState('');
  const [checking, setChecking] = useState(true);
  const [submitting, setSubmitting] = useState(false);

  // Check auth state on mount. Also re-checkable when triggered by the
  // 401 interceptor (session expired mid-session).
  const checkAuth = useCallback(async () => {
    setChecking(true);
    setUser(null);
    setMode(null);
    setError('');
    try {
      const me = await fetchMe();
      // Authenticated — either loopback passthrough or valid session.
      setUser(me);
    } catch (e: unknown) {
      // Not authenticated — determine whether we need setup or login.
      if (e && typeof e === 'object' && 'response' in e) {
        const status = (e as { response: { status?: number } }).response?.status;
        if (status === 401) {
          // Check if this is first-run (no users yet).
          try {
            await setup('', '');  // will 400, we just want to distinguish 409
          } catch (setupErr: unknown) {
            if (setupErr && typeof setupErr === 'object' && 'response' in setupErr) {
              const setupStatus = (setupErr as { response: { status?: number } }).response?.status;
              if (setupStatus === 409) {
                setMode('login');
              } else {
                setMode('setup');
              }
            } else {
              setMode('login'); // fallback
            }
          }
        } else {
          setError('Could not reach server');
        }
      } else {
        setError('Could not reach server');
      }
    } finally {
      setChecking(false);
    }
  }, [setUser]);

  // Register the 401 interceptor callback so expired sessions kick us back.
  useEffect(() => {
    return onAuthLost(() => {
      checkAuth();
    });
  }, [checkAuth]);

  // Initial check on mount.
  useEffect(() => {
    checkAuth();
  // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  // ─── Form handlers ───

  const handleSetup = async (username: string, password: string) => {
    setSubmitting(true);
    setError('');
    try {
      await setup(username, password);
      // Setup auto-logs in. Re-check to get the session.
      await checkAuth();
    } catch (e: unknown) {
      const msg = e && typeof e === 'object' && 'response' in e
        ? (e as { response: { data: { error?: string } } }).response?.data?.error
        : undefined;
      setError(msg || 'Setup failed');
    } finally {
      setSubmitting(false);
    }
  };

  const handleLogin = async (username: string, password: string) => {
    setSubmitting(true);
    setError('');
    try {
      await login(username, password);
      // Login sets the session cookie. Re-check to confirm.
      await checkAuth();
    } catch (e: unknown) {
      const msg = e && typeof e === 'object' && 'response' in e
        ? (e as { response: { data: { error?: string } } }).response?.data?.error
        : undefined;
      setError(msg || 'Login failed');
    } finally {
      setSubmitting(false);
    }
  };

  // ─── Render ───

  // Authenticated — render the main app. Children are already inside the
  // AuthProvider (see LoginScreen), so they can call useAuth().
  if (user) {
    return <>{children}</>;
  }

  // Checking auth state on mount.
  if (checking) {
    return (
      <div className="login-screen">
        <div className="login-card">
          <M3FadeIn>
            <div className="login-brand">
              <div className="login-brand-mark">
                <BrandMarkIcon />
              </div>
              <span className="login-brand-name">Entropy</span>
            </div>
            <div className="login-loading">
              <M3CircularProgress size={32} />
            </div>
          </M3FadeIn>
        </div>
      </div>
    );
  }

  // Network error (couldn't reach server at all).
  if (!mode && error) {
    return (
      <div className="login-screen">
        <div className="login-card">
          <M3FadeIn>
            <div className="login-brand">
              <div className="login-brand-mark">
                <BrandMarkIcon />
              </div>
              <span className="login-brand-name">Entropy</span>
            </div>
            <div className="login-error">{error}</div>
          </M3FadeIn>
        </div>
      </div>
    );
  }

  // Setup or login form.
  return (
    <div className="login-screen">
      <AnimatePresence mode="wait">
        <M3FadeIn key={mode}>
          <div className="login-card">
            <div className="login-brand">
              <div className="login-brand-mark">
                <BrandMarkIcon />
              </div>
              <span className="login-brand-name">Entropy</span>
            </div>

            <h1 className="login-heading">
              {mode === 'setup' ? 'Create admin account' : 'Sign in'}
            </h1>
            <p className="login-subheading">
              {mode === 'setup'
                ? 'Set up the first admin user for your Entropy instance.'
                : 'Enter your credentials to continue.'}
            </p>

            <AuthForm
              mode={mode!}
              onSubmit={mode === 'setup' ? handleSetup : handleLogin}
              submitting={submitting}
            />

            <AnimatePresence>
              {error && (
                <M3ScaleIn origin="top center">
                  <div className="login-error">{error}</div>
                </M3ScaleIn>
              )}
            </AnimatePresence>
          </div>
        </M3FadeIn>
      </AnimatePresence>
    </div>
  );
}

// ─── AuthForm — shared form for setup & login ───

function AuthForm({
  mode,
  onSubmit,
  submitting,
}: {
  mode: AuthMode;
  onSubmit: (username: string, password: string) => Promise<void>;
  submitting: boolean;
}) {
  const [username, setUsername] = useState('');
  const [password, setPassword] = useState('');
  const [confirmPassword, setConfirmPassword] = useState('');

  const canSubmit = username.length > 0
    && password.length >= 8
    && (mode === 'login' || password === confirmPassword);

  const handleSubmit = (e: React.FormEvent) => {
    e.preventDefault();
    if (!canSubmit || submitting) return;
    onSubmit(username.trim(), password);
  };

  return (
    <form className="login-form" onSubmit={handleSubmit}>
      <label className="login-label" htmlFor={mode === 'setup' ? 'setup-username' : 'login-username'}>
        Username
      </label>
      <input
        id={mode === 'setup' ? 'setup-username' : 'login-username'}
        className="login-input"
        type="text"
        autoComplete="username"
        autoFocus
        value={username}
        onChange={(e) => setUsername(e.target.value)}
        disabled={submitting}
        maxLength={64}
        placeholder={mode === 'setup' ? 'admin' : ''}
      />

      <label className="login-label" htmlFor={mode === 'setup' ? 'setup-password' : 'login-password'}>
        Password
      </label>
      <input
        id={mode === 'setup' ? 'setup-password' : 'login-password'}
        className="login-input"
        type="password"
        autoComplete={mode === 'setup' ? 'new-password' : 'current-password'}
        value={password}
        onChange={(e) => setPassword(e.target.value)}
        disabled={submitting}
        minLength={8}
        placeholder="At least 8 characters"
      />

      {mode === 'setup' && (
        <>
          <label className="login-label" htmlFor="setup-confirm-password">
            Confirm password
          </label>
          <input
            id="setup-confirm-password"
            className="login-input"
            type="password"
            autoComplete="new-password"
            value={confirmPassword}
            onChange={(e) => setConfirmPassword(e.target.value)}
            disabled={submitting}
            minLength={8}
            placeholder="Repeat your password"
          />
        </>
      )}

      <button
        className="login-submit"
        type="submit"
        disabled={!canSubmit || submitting}
      >
        {submitting ? (
          <M3CircularProgress size={20} strokeWidth={3} />
        ) : mode === 'setup' ? (
          'Create account'
        ) : (
          'Sign in'
        )}
      </button>
    </form>
  );
}
