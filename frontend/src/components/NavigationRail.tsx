import { M3NavRailIndicator } from './m3';
import { Ripple } from './Ripple';
import { useAuth } from '../lib/auth-context';
import BrandMarkIcon from './BrandMarkIcon';

type ViewId = 'search' | 'queue' | 'library' | 'settings' | 'users' | 'log';

interface NavigationRailProps {
  active: ViewId;
  onChange: (destination: ViewId) => void;
  queueCount?: number; // badge count for queue
}

// The full set of destinations. The "users" entry is filtered out at render
// time for non-admins and in loopback mode (see below), so the nav-rail
// indicator stays correctly positioned by index.
const ALL_DESTINATIONS = [
  { id: 'search', icon: 'search', label: 'Search' },
  { id: 'queue', icon: 'download', label: 'Queue' },
  { id: 'library', icon: 'library_music', label: 'Library' },
  { id: 'settings', icon: 'settings', label: 'Settings' },
  { id: 'users', icon: 'manage_accounts', label: 'Users', adminOnly: true },
  { id: 'log', icon: 'terminal', label: 'Log' },
] as const;

export default function NavigationRail({ active, onChange, queueCount }: NavigationRailProps) {
  const { user } = useAuth();

  // Show "Users" only to admins in non-loopback mode. Loopback mode has no
  // user management (the backend returns 400), and regular users shouldn't
  // see it even if the backend would reject them.
  const canManageUsers = !!user?.is_admin && !user?.loopback;
  const destinations = ALL_DESTINATIONS.filter(
    (d) => !('adminOnly' in d && d.adminOnly) || canManageUsers,
  );

  const activeIndex = destinations.findIndex((d) => d.id === active);

  return (
    <nav className="nav-rail" aria-label="Main navigation">
      <div className="nav-rail-brand" title="Entropy DL">
        <div className="brand-mark">
          <BrandMarkIcon />
        </div>
      </div>

      <div className="nav-rail-destinations">
        {/* Animated indicator pill */}
        <M3NavRailIndicator
          activeIndex={Math.max(0, activeIndex)}
          count={destinations.length}
          itemHeight={56}
          gap={4}
          paddingTop={0}
        />

        {destinations.map((dest) => {
          const isActive = active === dest.id;
          const showBadge = dest.id === 'queue' && queueCount !== undefined && queueCount > 0;

          return (
            <button
              key={dest.id}
              className={`nav-rail-item${isActive ? ' active' : ''}`}
              onClick={() => onChange(dest.id)}
              aria-current={isActive ? 'page' : undefined}
              aria-label={dest.label}
              type="button"
            >
              <span className="nav-rail-icon" aria-hidden="true">
                {dest.icon}
              </span>
              {showBadge && (
                <span className="nav-rail-badge" aria-label={`${queueCount} items`}>
                  {queueCount > 99 ? '99+' : queueCount}
                </span>
              )}
              <span className="nav-rail-label">{dest.label}</span>
              <Ripple />
            </button>
          );
        })}
      </div>
    </nav>
  );
}
