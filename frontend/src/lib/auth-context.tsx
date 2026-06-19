import { createContext, useContext, useState, type ReactNode } from 'react';
import type { AuthUser } from '../types';

// AuthContext carries the authenticated user (or null) to any descendant
// component. It is provided by LoginScreen, which owns the auth-check lifecycle,
// and consumed by things that need to know who is logged in or whether we're in
// loopback mode — e.g. NavigationRail (to show/hide admin-only destinations)
// and UsersPanel.
//
// Kept intentionally minimal: just { user, setUser }. Anything richer (logout
// action, re-check trigger) stays in LoginScreen, where the HTTP lifecycle lives.

interface AuthContextValue {
  user: AuthUser | null;
  setUser: (u: AuthUser | null) => void;
}

const AuthContext = createContext<AuthContextValue | null>(null);

export function AuthProvider({ children }: { children: ReactNode }) {
  const [user, setUser] = useState<AuthUser | null>(null);
  return (
    <AuthContext.Provider value={{ user, setUser }}>
      {children}
    </AuthContext.Provider>
  );
}

export function useAuth(): AuthContextValue {
  const ctx = useContext(AuthContext);
  if (!ctx) {
    throw new Error('useAuth must be used within an AuthProvider');
  }
  return ctx;
}
