import React, { useState, useEffect } from 'react';
import { showToast } from './Toast';
import { playClickSound, playSuccessSound } from '../utils/audio';

export default function SettingsModal({ onClose }) {
  const [soundEnabled, setSoundEnabled] = useState(() => localStorage.getItem('tactical_audio') === 'true');
  const [scanlinesEnabled, setScanlinesEnabled] = useState(() => document.body.classList.contains('scanlines-active'));
  const [dataSharing, setDataSharing] = useState(() => localStorage.getItem('global_threat_sharing') !== 'false');
  const [heuristicsLevel, setHeuristicsLevel] = useState(() => localStorage.getItem('heuristics_level') || 'Aggressive');

  const handleSave = (e) => {
    e.preventDefault();
    
    // Save to localStorage
    localStorage.setItem('tactical_audio', soundEnabled);
    localStorage.setItem('global_threat_sharing', dataSharing);
    localStorage.setItem('heuristics_level', heuristicsLevel);

    // Process scanlines
    if (scanlinesEnabled) {
      document.body.classList.add('scanlines-active');
    } else {
      document.body.classList.remove('scanlines-active');
    }

    if (soundEnabled) {
      setTimeout(() => playSuccessSound(), 100);
      showToast('Audio feedback matrix engaged.');
    }

    showToast('System configuration updated successfully.');
    onClose();
  };

  const handleToggle = (setter, current) => {
    playClickSound();
    setter(!current);
  };

  return (
    <div className="modal-overlay fade-in-up" onClick={onClose} style={{
      position: 'fixed', top: 0, left: 0, width: '100%', height: '100%',
      background: 'rgba(2, 6, 23, 0.8)', backdropFilter: 'blur(12px)',
      display: 'flex', alignItems: 'center', justifyContent: 'center', zIndex: 1000
    }}>
      <div className="glass-panel modal-panel emerald-glow-strong" onClick={e => e.stopPropagation()} style={{
        width: '100%', maxWidth: '480px', padding: '2rem', position: 'relative'
      }}>
        {/* Header */}
        <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: '2rem' }}>
          <div style={{ display: 'flex', alignItems: 'center', gap: '0.75rem' }}>
            <span className="material-symbols-outlined" style={{ fontSize: '24px', color: 'var(--primary-container)' }}>settings</span>
            <h2 style={{ fontFamily: 'var(--font-headline)', fontSize: '1.5rem', color: 'var(--primary)' }}>
              System Configuration
            </h2>
          </div>
          <button onClick={onClose} style={{ background: 'none', border: 'none', color: 'var(--outline)', cursor: 'pointer' }}>
            <span className="material-symbols-outlined">close</span>
          </button>
        </div>

        {/* Form */}
        <form onSubmit={handleSave}>
          <div className="label" style={{ color: 'var(--outline)', marginBottom: '1rem' }}>Display & Audio Matrix</div>
          
          <label className="custom-checkbox" style={{ marginBottom: '1rem', display: 'flex', alignItems: 'center', gap: '0.75rem', cursor: 'pointer' }}>
            <input type="checkbox" checked={soundEnabled} onChange={() => handleToggle(setSoundEnabled, soundEnabled)} />
            <span className="checkbox-mark">
              <span className="material-symbols-outlined">check</span>
            </span>
            <span style={{ fontSize: '0.875rem', color: soundEnabled ? 'var(--primary-container)' : 'var(--outline)' }}>
              Enable Tactical Audio Feedback
            </span>
          </label>

          <label className="custom-checkbox" style={{ marginBottom: '2rem', display: 'flex', alignItems: 'center', gap: '0.75rem', cursor: 'pointer' }}>
            <input type="checkbox" checked={scanlinesEnabled} onChange={() => handleToggle(setScanlinesEnabled, scanlinesEnabled)} />
            <span className="checkbox-mark">
              <span className="material-symbols-outlined">check</span>
            </span>
            <span style={{ fontSize: '0.875rem', color: scanlinesEnabled ? 'var(--primary-container)' : 'var(--outline)' }}>
              CRT Scanline Overlay
            </span>
          </label>

          <div className="label" style={{ color: 'var(--outline)', marginBottom: '1rem' }}>Core Engine Tuning</div>

          <div style={{ position: 'relative', marginBottom: '1.5rem' }}>
            <span className="material-symbols-outlined" style={{ position: 'absolute', left: '0', top: '50%', transform: 'translateY(-50%)', fontSize: '18px', color: 'var(--primary-container)' }}>filter_alt</span>
            <select 
              className="input-underline" 
              style={{ appearance: 'none', paddingLeft: '32px' }}
              value={heuristicsLevel}
              onChange={e => { playClickSound(); setHeuristicsLevel(e.target.value); }}
            >
              <option value="Standard" style={{ background: '#121416', color: '#e2e2e5' }}>Standard Analysis Mode</option>
              <option value="Aggressive" style={{ background: '#121416', color: '#e2e2e5' }}>Aggressive Heuristics Mode</option>
              <option value="Paranoid" style={{ background: '#121416', color: '#e2e2e5' }}>Paranoid Zero-Trust Mode</option>
            </select>
            <span className="material-symbols-outlined" style={{ position: 'absolute', right: '0', top: '50%', transform: 'translateY(-50%)', color: 'var(--outline)', pointerEvents: 'none' }}>expand_more</span>
          </div>

          <label className="custom-checkbox" style={{ marginBottom: '2.5rem', display: 'flex', alignItems: 'center', gap: '0.75rem', cursor: 'pointer' }}>
            <input type="checkbox" checked={dataSharing} onChange={() => handleToggle(setDataSharing, dataSharing)} />
            <span className="checkbox-mark">
              <span className="material-symbols-outlined">check</span>
            </span>
            <span style={{ fontSize: '0.875rem', color: dataSharing ? 'var(--primary-container)' : 'var(--outline)' }}>
              Participate in Global Threat Intel Network
            </span>
          </label>

          <div style={{ display: 'flex', gap: '1rem', justifyContent: 'flex-end' }}>
            <button type="button" className="btn btn-outline" style={{ cursor: 'pointer' }} onClick={onClose}>
              Abort
            </button>
            <button type="submit" className="btn btn-primary btn-shine" style={{ cursor: 'pointer' }}>
              Confirm Reboot
            </button>
          </div>
        </form>
      </div>
    </div>
  );
}
