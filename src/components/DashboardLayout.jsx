import React, { useState, useEffect } from 'react';
import { Outlet, NavLink, useNavigate, useLocation } from 'react-router-dom';
import LogoIcon from './LogoIcon';
import { showToast } from './Toast';
import { generateAlerts } from '../data/mockData';
import { playAlertSound, playNotificationSound, playClickSound } from '../utils/audio';
import ProfileModal from './ProfileModal';
import SettingsModal from './SettingsModal';
const NAV_ITEMS = [
  { to: '/overview', icon: 'dashboard', label: 'Overview' },
  { to: '/clients', icon: 'group', label: 'Clients' },
  { to: '/alerts', icon: 'notifications_active', label: 'Alerts' },
  { to: '/analytics', icon: 'insights', label: 'Analytics' },
  { to: '/simulator', icon: 'swords', label: 'Attack Simulator' },
];

export default function DashboardLayout() {
  const [dropdownOpen, setDropdownOpen] = useState(false);
  const [isProfileModalOpen, setIsProfileModalOpen] = useState(false);
  const [isSettingsModalOpen, setIsSettingsModalOpen] = useState(false);
  const [notificationsOpen, setNotificationsOpen] = useState(false);
  const [notifications, setNotifications] = useState(() => generateAlerts(5));
  const navigate = useNavigate();
  const location = useLocation();

  useEffect(() => {
    // Generate new alert every 15s to simulate real-time notifications
    const iv = setInterval(() => {
      const newAlert = generateAlerts(1)[0];
      setNotifications(prev => [newAlert, ...prev].slice(0, 15));
      
      // Optionally show a toast for high-severity alerts when not on the alerts page
      if (newAlert.severity === 'critical') {
        playAlertSound();
        if (location.pathname !== '/alerts') {
          showToast(`CRITICAL ALERT: ${newAlert.title}`);
        }
        
        // Check if data sharing is active
        if (localStorage.getItem('global_threat_sharing') !== 'false') {
          setTimeout(() => {
            showToast('Threat metadata synchronized with Global Intel Network');
          }, 2500);
        }
      } else {
        playNotificationSound();
      }
    }, 15000);
    return () => clearInterval(iv);
  }, [location.pathname]);

  return (
    <div className="page-container">
      {/* Sidebar */}
      <aside className="sidebar">
        <div style={{ marginBottom: '0.5rem' }}>
          <div className="sidebar-logo">
            <LogoIcon size={36} color="#00ff9d" />
            <span className="sidebar-logo-text">ShieldProxy</span>
          </div>
          <div className="label" style={{ color: 'var(--outline)', marginTop: '0.25rem', marginLeft: '0.25rem' }}>
            Vanguard Level
          </div>
        </div>

        <nav className="sidebar-nav">
          {NAV_ITEMS.map((item) => (
            <NavLink
              key={item.to}
              to={item.to}
              className={({ isActive }) => `sidebar-link${isActive ? ' active' : ''}`}
            >
              <span className="material-symbols-outlined" style={{ fontSize: '20px' }}>
                {item.icon}
              </span>
              {item.label}
            </NavLink>
          ))}
        </nav>

        <button
          className="btn btn-primary btn-shine"
          style={{ width: '100%', marginTop: '1rem' }}
          onClick={() => showToast('Deploying new Sentry node to the edge...')}
        >
          <span className="material-symbols-outlined" style={{ fontSize: '18px' }}>rocket_launch</span>
          Deploy Sentry
        </button>
      </aside>

      {/* Top Bar */}
      <header className="topbar">
        <div className="topbar-left">
          <div className="pulse-dot-sm" />
          <span className="label" style={{ color: 'var(--primary-container)' }}>Secure Orbit</span>
          <span className="label" style={{ color: 'var(--outline)' }}>System Status:</span>
          <span className="label" style={{ color: 'var(--primary-container)' }}>Online</span>
        </div>

        <div className="topbar-center">
          <input
            type="text"
            className="topbar-search"
            placeholder="QUERY NETWORK..."
            onKeyDown={(e) => { if (e.key === 'Enter') showToast(`Searching network for: ${e.target.value}`) }}
          />
        </div>

        <div className="topbar-right">
          <div style={{ position: 'relative' }}>
            <button className="topbar-icon-btn" onClick={() => { playClickSound(); setNotificationsOpen(!notificationsOpen); setDropdownOpen(false); }}>
              <span className="material-symbols-outlined">notifications</span>
              {notifications.length > 0 && <span className="notification-badge" />}
            </button>
            {notificationsOpen && (
              <div className="user-dropdown fade-in-up" style={{ width: '340px', right: '-10px', padding: 0, overflow: 'hidden' }}>
                <div style={{ padding: '1rem', borderBottom: '1px solid rgba(0, 255, 157, 0.1)', display: 'flex', justifyContent: 'space-between', alignItems: 'center', background: 'rgba(2, 6, 23, 0.8)' }}>
                  <span style={{ fontFamily: 'var(--font-headline)', color: 'var(--primary)', fontSize: '1.25rem' }}>Alerts</span>
                  <button onClick={() => setNotifications([])} style={{ background: 'none', border: 'none', color: 'var(--outline)', fontSize: '11px', cursor: 'pointer', transition: 'color 0.2s' }} onMouseOver={e => e.target.style.color='var(--error)'} onMouseOut={e => e.target.style.color='var(--outline)'}>Clear All</button>
                </div>
                <div style={{ maxHeight: '320px', overflowY: 'auto', background: 'rgba(2, 6, 23, 0.9)' }}>
                  {notifications.length === 0 ? (
                    <div style={{ padding: '3rem 1rem', textAlign: 'center', color: 'var(--outline)', fontSize: '13px' }}>
                      <span className="material-symbols-outlined" style={{ display: 'block', fontSize: '32px', marginBottom: '0.5rem', opacity: 0.5 }}>notifications_paused</span>
                      No new alerts
                    </div>
                  ) : (
                    notifications.map(notif => (
                      <div key={notif.id} style={{ padding: '1rem', borderBottom: '1px solid rgba(0, 255, 157, 0.05)', display: 'flex', gap: '0.75rem', cursor: 'pointer', transition: 'background 0.2s', background: notif.severity === 'critical' ? 'rgba(255, 180, 171, 0.05)' : 'transparent' }} 
                           onClick={() => { setNotificationsOpen(false); navigate('/alerts'); }}
                           onMouseOver={e => e.currentTarget.style.background = 'rgba(0, 255, 157, 0.05)'}
                           onMouseOut={e => e.currentTarget.style.background = notif.severity === 'critical' ? 'rgba(255, 180, 171, 0.05)' : 'transparent'}>
                        <div style={{ width: '4px', background: notif.severity === 'critical' ? 'var(--error)' : notif.severity === 'high' ? 'var(--tertiary-fixed)' : 'var(--secondary-container)', flexShrink: 0, borderRadius: '2px' }} />
                        <div style={{ flex: 1 }}>
                          <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'flex-start', marginBottom: '0.25rem' }}>
                            <span style={{ color: notif.severity === 'critical' ? 'var(--error)' : 'var(--primary-container)', fontSize: '13px', fontWeight: 'bold', lineHeight: 1.2 }}>{notif.title}</span>
                            <span style={{ color: 'var(--outline)', fontSize: '10px', whiteSpace: 'nowrap', marginLeft: '0.5rem' }}>{notif.timestamp.split(' ')[1]}</span>
                          </div>
                          <div style={{ color: 'var(--on-surface-variant)', fontSize: '12px', lineHeight: 1.4, WebkitLineClamp: 2, WebkitBoxOrient: 'vertical', display: '-webkit-box', overflow: 'hidden' }}>{notif.description}</div>
                        </div>
                      </div>
                    ))
                  )}
                </div>
                <div style={{ padding: '0.75rem', textAlign: 'center', borderTop: '1px solid rgba(0, 255, 157, 0.1)', background: 'rgba(2, 6, 23, 0.95)' }}>
                  <button onClick={() => { setNotificationsOpen(false); navigate('/alerts'); }} style={{ background: 'none', border: 'none', color: 'var(--primary)', fontSize: '13px', cursor: 'pointer', fontFamily: 'var(--font-headline)', width: '100%' }}>View All Alerts</button>
                </div>
              </div>
            )}
          </div>
          <button className="topbar-icon-btn" onClick={() => { playClickSound(); setIsSettingsModalOpen(true); setDropdownOpen(false); setNotificationsOpen(false); }}>
            <span className="material-symbols-outlined">settings</span>
          </button>

          <div className="topbar-user">
            <button
              className="topbar-avatar"
              onClick={() => { playClickSound(); setDropdownOpen(!dropdownOpen); setNotificationsOpen(false); }}
              style={{ border: 'none', cursor: 'none' }}
            >
              OP
            </button>
            <div className="topbar-user-info">
              <span className="topbar-user-name">Operator_01</span>
              <span className="topbar-user-level">SECURE_LEVEL_7</span>
            </div>

            {dropdownOpen && (
              <div className="user-dropdown">
                <button className="dropdown-item" onClick={() => { setDropdownOpen(false); setIsProfileModalOpen(true); }}>
                  <span className="material-symbols-outlined" style={{ fontSize: '16px' }}>person</span>
                  Profile Setup
                </button>
                <button
                  className="dropdown-item danger"
                  onClick={() => {
                    setDropdownOpen(false);
                    showToast('Terminating Vault Session.');
                    navigate('/');
                  }}
                >
                  <span className="material-symbols-outlined" style={{ fontSize: '16px' }}>logout</span>
                  Terminate Session
                </button>
              </div>
            )}
          </div>
        </div>
      </header>

      {/* Content */}
      <main className="dashboard-content fade-in-up">
        <Outlet />
      </main>

      {/* Profile Modal */}
      {isProfileModalOpen && (
        <ProfileModal onClose={() => setIsProfileModalOpen(false)} />
      )}

      {/* Settings Modal */}
      {isSettingsModalOpen && (
        <SettingsModal onClose={() => setIsSettingsModalOpen(false)} />
      )}
    </div>
  );
}
