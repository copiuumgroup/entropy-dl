import { M3NavRailIndicator } from './m3';
import { Ripple } from './Ripple';
import BrandMarkIcon from './BrandMarkIcon';

type ViewId = 'search' | 'queue' | 'settings' | 'log';

interface NavigationRailProps {
  active: ViewId;
  onChange: (destination: ViewId) => void;
  queueCount?: number; // badge count for queue
}

const DESTINATIONS = [
  { id: 'search', icon: 'search', label: 'Search' },
  { id: 'queue', icon: 'download', label: 'Queue' },
  { id: 'settings', icon: 'settings', label: 'Settings' },
  { id: 'log', icon: 'terminal', label: 'Log' },
] as const;

export default function NavigationRail({ active, onChange, queueCount }: NavigationRailProps) {
  const activeIndex = DESTINATIONS.findIndex((d) => d.id === active);

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
          count={DESTINATIONS.length}
          itemHeight={56}
          gap={4}
          paddingTop={0}
        />

        {DESTINATIONS.map((dest) => {
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