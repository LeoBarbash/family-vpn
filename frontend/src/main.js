const $ = (id) => document.getElementById(id);

const el = {
  profileName: $('profile-name'),
  config:      $('config'),
  connect:     $('connect'),
  disconnect:  $('disconnect'),
  loadExample: $('load-example'),
  message:     $('message'),
  statusPill:  $('status-pill'),
  endpoint:    $('endpoint'),
  iface:       $('interface'),
  since:       $('since'),
};

/* ------------------------------------------------------------------ */
/* Wails runtime check                                                */
/* ------------------------------------------------------------------ */

function wailsReady() {
  return !!(window.go && window.go.main && window.go.main.App);
}

function wails() {
  return window.go.main.App;
}

/* ------------------------------------------------------------------ */
/* UI helpers                                                         */
/* ------------------------------------------------------------------ */

function setMessage(text, type = 'info') {
  el.message.textContent = text || '';
  const colours = {
    info:    '#fbbf24',
    error:   '#f87171',
    success: '#86efac',
  };
  el.message.style.color = colours[type] || colours.info;
}

function updateStatus(s) {
  if (!s) return;

  const connected = s.connected;
  el.statusPill.textContent = connected ? 'Connected' : 'Disconnected';
  el.statusPill.className   = 'pill ' + (connected ? 'pill-on' : 'pill-off');

  el.endpoint.textContent = s.endpoint || '—';
  el.iface.textContent    = s.interface || '—';
  el.since.textContent    = s.since
    ? new Date(s.since).toLocaleString()
    : '—';

  el.connect.disabled    = connected;
  el.disconnect.disabled = !connected;

  if (s.message && s.message !== 'connected' && s.message !== 'disconnected') {
    setMessage(s.message, connected ? 'success' : 'error');
  } else {
    setMessage('');
  }
}

/* ------------------------------------------------------------------ */
/* Actions                                                            */
/* ------------------------------------------------------------------ */

async function doLoadExample() {
  if (!wailsReady()) {
    setMessage('Wails runtime not found — open this app with wails dev or the built binary.', 'error');
    return;
  }
  try {
    const cfg = await wails().ExampleConfig();
    el.config.value = cfg;
    setMessage('Example config loaded.', 'success');
  } catch (err) {
    setMessage('Error: ' + err, 'error');
  }
}

async function doConnect() {
  if (!wailsReady()) {
    setMessage('Wails runtime not found.', 'error');
    return;
  }

  const name   = el.profileName.value.trim() || 'family';
  const rawCfg = el.config.value.trim();

  if (!rawCfg) {
    setMessage('Paste your wg-quick config first.', 'error');
    el.config.focus();
    return;
  }

  setMessage('Parsing config…', 'info');
  try {
    await wails().LoadConfig(name, rawCfg);
  } catch (err) {
    setMessage('Config error: ' + err, 'error');
    return;
  }

  setMessage('Connecting…', 'info');
  try {
    const status = await wails().Connect();
    updateStatus(status);
  } catch (err) {
    // Even on failure the backend may have updated status / lastError
    refreshStatus();
    setMessage('Connection failed: ' + err, 'error');
  }
}

async function doDisconnect() {
  if (!wailsReady()) return;

  setMessage('Disconnecting…', 'info');
  try {
    const status = await wails().Disconnect();
    updateStatus(status);
  } catch (err) {
    setMessage('Disconnect error: ' + err, 'error');
  }
}

async function refreshStatus() {
  if (!wailsReady()) return;
  try {
    const s = await wails().Status();
    updateStatus(s);
  } catch (err) {
    console.error('status poll error:', err);
  }
}

/* ------------------------------------------------------------------ */
/* Drag-and-drop config file                                          */
/* ------------------------------------------------------------------ */

window.addEventListener('dragover', (e) => {
  e.preventDefault();
  el.config.style.borderColor = '#5b8cff';
});

window.addEventListener('dragleave', (e) => {
  e.preventDefault();
  el.config.style.borderColor = '';
});

window.addEventListener('drop', async (e) => {
  e.preventDefault();
  el.config.style.borderColor = '';

  const file = e.dataTransfer.files[0];
  if (!file) return;

  // If running inside Wails we can hand the path to Go directly.
  if (wailsReady() && file.path) {
    try {
      const info = await wails().LoadConfigFromFile(file.path);
      el.profileName.value = info.name || 'family';
      setMessage('Loaded "' + (info.name || file.name) + '"', 'success');
      // Also populate the textarea so the user sees what was loaded
      const text = await file.text();
      el.config.value = text;
    } catch (err) {
      setMessage('File error: ' + err, 'error');
    }
    return;
  }

  // Fallback for regular browser / no Wails runtime
  try {
    const text = await file.text();
    el.config.value = text;
    el.profileName.value = file.name.replace(/\.conf$/i, '') || 'family';
    setMessage('Loaded "' + file.name + '"', 'success');
  } catch (err) {
    setMessage('Read error: ' + err, 'error');
  }
});

/* ------------------------------------------------------------------ */
/* Keyboard shortcut: Ctrl/Cmd + Enter to connect                     */
/* ------------------------------------------------------------------ */

el.config.addEventListener('keydown', (e) => {
  if ((e.ctrlKey || e.metaKey) && e.key === 'Enter') {
    e.preventDefault();
    doConnect();
  }
});

/* ------------------------------------------------------------------ */
/* Bind buttons                                                       */
/* ------------------------------------------------------------------ */

el.loadExample.addEventListener('click', doLoadExample);
el.connect.addEventListener('click', doConnect);
el.disconnect.addEventListener('click', doDisconnect);

/* ------------------------------------------------------------------ */
/* Polling & init                                                     */
/* ------------------------------------------------------------------ */

setInterval(refreshStatus, 3000);
refreshStatus();
