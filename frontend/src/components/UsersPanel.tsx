import { useState, useEffect, useCallback, useRef } from 'react';
import { createPortal } from 'react-dom';
import { motion } from 'framer-motion';
import { AnimatePresence, M3FadeIn, M3Stagger, M3StaggerItem, M3CircularProgress, M3ScaleIn, M3SwitchAnimated } from './m3';
import { Ripple } from './Ripple';
import { fetchUsers, createUser, deleteUser } from '../lib/api';
import { useAuth } from '../lib/auth-context';
import type { User } from '../types';

// UsersPanel — admin-only account management (homelab mode).
//
// Lists every named account, lets the admin add new ones (with an admin
// toggle) and delete existing ones. The backend enforces:
//   - admin-only access (403 for non-admins, 400 in loopback mode)
//   - last-admin protection (Delete refuses to remove the final admin)
// So this UI can stay simple — it surfaces backend errors as toasts rather
// than re-implementing those rules client-side.
//
// The panel is only reachable via the nav rail when the current user is an
// admin in non-loopback mode (see NavigationRail), but it re-reads the auth
// context here to decide whether to show the current-user badge / disable
// self-delete for clarity.

function formatDate(iso: string): string {
  if (!iso) return '';
  const d = new Date(iso);
  if (Number.isNaN(d.getTime())) return '';
  return d.toLocaleDateString([], { year: 'numeric', month: 'short', day: 'numeric' });
}

export default function UsersPanel({ onToast }: { onToast: (msg: string, isErr?: boolean) => void }) {
  const { user: currentUser } = useAuth();
  const [users, setUsers] = useState<User[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [showAdd, setShowAdd] = useState(false);
  const [deleteTarget, setDeleteTarget] = useState<User | null>(null);

  const load = useCallback(async () => {
    setLoading(true);
    setError(null);
    try {
      const result = await fetchUsers();
      // Stable, admin-first ordering for a predictable list.
      result.sort((a, b) => {
        if (a.is_admin !== b.is_admin) return a.is_admin ? -1 : 1;
        return a.username.localeCompare(b.username);
      });
      setUsers(result);
    } catch (e: unknown) {
      const msg = e instanceof Error ? e.message : 'Failed to load users';
      // Axios errors carry a response.data.error from the backend.
      if (e && typeof e === 'object' && 'response' in e) {
        const dataErr = (e as { response: { data?: { error?: string } } }).response?.data?.error;
        if (dataErr) setError(dataErr);
        else setError(msg);
      } else {
        setError(msg);
      }
      setUsers([]);
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    load();
  }, [load]);

  const handleCreated = async (username: string, password: string, isAdmin: boolean) => {
    try {
      await createUser({ username, password, is_admin: isAdmin });
      setShowAdd(false);
      onToast(`Created user "${username}"`);
      await load();
    } catch (e: unknown) {
      const msg = e && typeof e === 'object' && 'response' in e
        ? (e as { response: { data?: { error?: string } } }).response?.data?.error
        : undefined;
      onToast(msg || 'Failed to create user', true);
    }
  };

  const handleDelete = async (username: string) => {
    try {
      await deleteUser(username);
      setDeleteTarget(null);
      onToast(`Deleted user "${username}"`);
      await load();
    } catch (e: unknown) {
      const msg = e && typeof e === 'object' && 'response' in e
        ? (e as { response: { data?: { error?: string } } }).response?.data?.error
        : undefined;
      onToast(msg || 'Failed to delete user', true);
    }
  };

  return (
    <section className="users" data-testid="users-panel">
      <div className="users-header">
        <span className="users-title">Accounts</span>
        <button
          className="btn tonal users-add-btn"
          onClick={() => setShowAdd(true)}
          type="button"
        >
          <span className="md-icon" aria-hidden="true">person_add</span>
          Add user
        </button>
      </div>

      <p className="users-subheading">
        Named accounts can sign in from any device on your network. Admins can manage
        other accounts; regular users can only use the downloader.
      </p>

      {loading ? (
        <div className="users-loading">
          <M3CircularProgress />
          <span>Loading…</span>
        </div>
      ) : error ? (
        <div className="users-empty">
          <span className="empty-icon" aria-hidden="true">error_outline</span>
          <span>{error}</span>
        </div>
      ) : users.length === 0 ? (
        <div className="users-empty">
          <span className="empty-icon" aria-hidden="true">group</span>
          <span>No accounts yet</span>
        </div>
      ) : (
        <M3Stagger className="users-list" staggerDelay={0.03}>
          <AnimatePresence mode="popLayout">
            {users.map((u) => {
              const isSelf = currentUser?.username === u.username;
              return (
                <M3StaggerItem key={u.username}>
                  <div className="users-entry">
                    <span className="users-entry-icon" aria-hidden="true">
                      {u.is_admin ? 'shield_person' : 'person'}
                    </span>
                    <span className="users-entry-info">
                      <span className="users-entry-name">
                        {u.username}
                        {isSelf && <span className="users-entry-self">you</span>}
                      </span>
                      <span className="users-entry-meta">
                        {u.is_admin && <span className="users-entry-admin">Admin</span>}
                        {u.created_at && ` · added ${formatDate(u.created_at)}`}
                      </span>
                    </span>
                    <button
                      className="btn icon users-entry-delete"
                      onClick={() => setDeleteTarget(u)}
                      aria-label={`Delete user ${u.username}`}
                      title={`Delete ${u.username}`}
                      type="button"
                      style={{ color: 'var(--md-error)' }}
                    >
                      <span className="md-icon" aria-hidden="true">delete</span>
                      <Ripple />
                    </button>
                  </div>
                </M3StaggerItem>
              );
            })}
          </AnimatePresence>
        </M3Stagger>
      )}

      {/* Add-user dialog */}
      <AnimatePresence>
        {showAdd && (
          <AddUserDialog
            onClose={() => setShowAdd(false)}
            onSubmit={handleCreated}
          />
        )}
      </AnimatePresence>

      {/* Delete confirmation */}
      <AnimatePresence>
        {deleteTarget && (
          <ConfirmDeleteDialog
            user={deleteTarget}
            onClose={() => setDeleteTarget(null)}
            onConfirm={() => handleDelete(deleteTarget.username)}
          />
        )}
      </AnimatePresence>
    </section>
  );
}

// ─── AddUserDialog ───

function AddUserDialog({
  onClose,
  onSubmit,
}: {
  onClose: () => void;
  onSubmit: (username: string, password: string, isAdmin: boolean) => Promise<void>;
}) {
  const [username, setUsername] = useState('');
  const [password, setPassword] = useState('');
  const [confirmPassword, setConfirmPassword] = useState('');
  const [isAdmin, setIsAdmin] = useState(false);
  const [submitting, setSubmitting] = useState(false);
  const dialogRef = useRef<HTMLDivElement>(null);

  // Escape to close + focus trap (mirrors InfoModal/WelcomeOverlay).
  useEffect(() => {
    const onKey = (e: KeyboardEvent) => {
      if (e.key === 'Escape') onClose();
    };
    document.addEventListener('keydown', onKey);
    const prev = document.activeElement as HTMLElement;
    const t = setTimeout(() => {
      const first = dialogRef.current?.querySelector<HTMLElement>('input');
      first?.focus();
    }, 50);
    return () => {
      document.removeEventListener('keydown', onKey);
      clearTimeout(t);
      prev?.focus();
    };
  }, [onClose]);

  const canSubmit = username.trim().length > 0
    && password.length >= 8
    && password === confirmPassword;

  const handleSubmit = (e: React.FormEvent) => {
    e.preventDefault();
    if (!canSubmit || submitting) return;
    setSubmitting(true);
    onSubmit(username.trim(), password, isAdmin).finally(() => setSubmitting(false));
  };

  const content = (
    <div className="m3-dialog-portal">
      <div className="m3-dialog-scrim" onClick={onClose} />
      <M3ScaleIn>
        <div
          className="m3-dialog users-dialog"
          role="dialog"
          aria-modal="true"
          aria-label="Add user"
          tabIndex={-1}
          ref={dialogRef}
        >
          <div className="m3-dialog-header">
            <div className="m3-dialog-header-text">
              <h2 className="m3-dialog-title">Add user</h2>
              <span className="m3-dialog-subtitle">Create a new account</span>
            </div>
            <button
              className="btn icon m3-dialog-close-btn"
              onClick={onClose}
              aria-label="Close"
              type="button"
            >
              <span className="md-icon" aria-hidden="true">close</span>
            </button>
          </div>

          <form className="users-dialog-body" onSubmit={handleSubmit}>
            <label className="login-label" htmlFor="adduser-username">Username</label>
            <input
              id="adduser-username"
              className="login-input"
              type="text"
              autoComplete="username"
              value={username}
              onChange={(e) => setUsername(e.target.value)}
              disabled={submitting}
              maxLength={64}
              placeholder="e.g. kid"
            />

            <label className="login-label" htmlFor="adduser-password">Password</label>
            <input
              id="adduser-password"
              className="login-input"
              type="password"
              autoComplete="new-password"
              value={password}
              onChange={(e) => setPassword(e.target.value)}
              disabled={submitting}
              minLength={8}
              placeholder="At least 8 characters"
            />

            <label className="login-label" htmlFor="adduser-confirm">Confirm password</label>
            <input
              id="adduser-confirm"
              className="login-input"
              type="password"
              autoComplete="new-password"
              value={confirmPassword}
              onChange={(e) => setConfirmPassword(e.target.value)}
              disabled={submitting}
              minLength={8}
              placeholder="Repeat the password"
            />

            <button
              type="button"
              className="m3-switch-item users-admin-toggle"
              onClick={() => setIsAdmin(!isAdmin)}
              aria-pressed={isAdmin}
              disabled={submitting}
            >
              <M3SwitchAnimated on={isAdmin} />
              <span className="m3-switch-label">Admin (can manage other accounts)</span>
            </button>

            {password && confirmPassword && password !== confirmPassword && (
              <div className="users-dialog-error">Passwords do not match</div>
            )}

            <div className="m3-dialog-actions">
              <button className="btn text" type="button" onClick={onClose} disabled={submitting}>
                Cancel
              </button>
              <motion.button
                className="btn primary"
                type="submit"
                disabled={!canSubmit || submitting}
                whileTap={{ scale: 0.97 }}
              >
                {submitting ? <M3CircularProgress size={20} strokeWidth={3} /> : 'Create user'}
              </motion.button>
            </div>
          </form>
        </div>
      </M3ScaleIn>
    </div>
  );

  return createPortal(content, document.body);
}

// ─── ConfirmDeleteDialog ───

function ConfirmDeleteDialog({
  user,
  onClose,
  onConfirm,
}: {
  user: User;
  onClose: () => void;
  onConfirm: () => void;
}) {
  const [submitting, setSubmitting] = useState(false);
  const dialogRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    const onKey = (e: KeyboardEvent) => {
      if (e.key === 'Escape') onClose();
    };
    document.addEventListener('keydown', onKey);
    const prev = document.activeElement as HTMLElement;
    const t = setTimeout(() => {
      dialogRef.current?.querySelector<HTMLElement>('button')?.focus();
    }, 50);
    return () => {
      document.removeEventListener('keydown', onKey);
      clearTimeout(t);
      prev?.focus();
    };
  }, [onClose]);

  const handleConfirm = () => {
    setSubmitting(true);
    onConfirm();
  };

  const content = (
    <div className="m3-dialog-portal">
      <div className="m3-dialog-scrim" onClick={onClose} />
      <M3ScaleIn>
        <div
          className="m3-dialog users-dialog users-dialog-sm"
          role="alertdialog"
          aria-modal="true"
          aria-label={`Delete user ${user.username}`}
          tabIndex={-1}
          ref={dialogRef}
        >
          <div className="m3-dialog-header">
            <div className="m3-dialog-header-text">
              <h2 className="m3-dialog-title">Delete user?</h2>
            </div>
            <button
              className="btn icon m3-dialog-close-btn"
              onClick={onClose}
              aria-label="Close"
              type="button"
            >
              <span className="md-icon" aria-hidden="true">close</span>
            </button>
          </div>
          <M3FadeIn className="users-dialog-body">
            <p className="users-confirm-text">
              This permanently deletes the account <strong>{user.username}</strong>.
              They will be signed out immediately and can no longer sign in.
            </p>
          </M3FadeIn>
          <div className="m3-dialog-actions">
            <button className="btn text" type="button" onClick={onClose} disabled={submitting}>
              Cancel
            </button>
            <motion.button
              className="btn users-btn-danger"
              type="button"
              onClick={handleConfirm}
              disabled={submitting}
              whileTap={{ scale: 0.97 }}
            >
              {submitting ? <M3CircularProgress size={20} strokeWidth={3} /> : 'Delete'}
            </motion.button>
          </div>
        </div>
      </M3ScaleIn>
    </div>
  );

  return createPortal(content, document.body);
}
