// audio.js - Synthesizes UI sounds using Web Audio API

let audioCtx = null;

function initAudio() {
  if (!audioCtx) {
    audioCtx = new (window.AudioContext || window.webkitAudioContext)();
  }
  if (audioCtx.state === 'suspended') {
    audioCtx.resume();
  }
}

function canPlay() {
  return localStorage.getItem('tactical_audio') === 'true';
}

function playTone(freq, type, duration, vol) {
  if (!canPlay()) return;
  initAudio();
  
  const oscillator = audioCtx.createOscillator();
  const gainNode = audioCtx.createGain();
  
  oscillator.type = type;
  oscillator.frequency.setValueAtTime(freq, audioCtx.currentTime);
  
  gainNode.gain.setValueAtTime(vol, audioCtx.currentTime);
  gainNode.gain.exponentialRampToValueAtTime(0.001, audioCtx.currentTime + duration);
  
  oscillator.connect(gainNode);
  gainNode.connect(audioCtx.destination);
  
  oscillator.start();
  oscillator.stop(audioCtx.currentTime + duration);
}

// Low electronic click for interactions (buttons)
export function playClickSound() {
  playTone(800, 'sine', 0.05, 0.1);
  setTimeout(() => playTone(1200, 'sine', 0.05, 0.05), 50);
}

// Success chime
export function playSuccessSound() {
  playTone(440, 'sine', 0.1, 0.1);
  setTimeout(() => playTone(554, 'sine', 0.1, 0.1), 100);
  setTimeout(() => playTone(659, 'sine', 0.3, 0.1), 200);
}

// Notification blip
export function playNotificationSound() {
  playTone(880, 'triangle', 0.1, 0.1);
  setTimeout(() => playTone(880, 'triangle', 0.15, 0.08), 150);
}

// Critical alert alarm
export function playAlertSound() {
  if (!canPlay()) return;
  initAudio();
  
  const now = audioCtx.currentTime;
  const oscillator = audioCtx.createOscillator();
  const gainNode = audioCtx.createGain();
  
  oscillator.type = 'sawtooth';
  // Siren effect
  oscillator.frequency.setValueAtTime(400, now);
  oscillator.frequency.linearRampToValueAtTime(800, now + 0.2);
  oscillator.frequency.linearRampToValueAtTime(400, now + 0.4);
  
  gainNode.gain.setValueAtTime(0.15, now);
  gainNode.gain.linearRampToValueAtTime(0, now + 0.5);
  
  oscillator.connect(gainNode);
  gainNode.connect(audioCtx.destination);
  
  oscillator.start(now);
  oscillator.stop(now + 0.5);
}
