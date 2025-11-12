(function () {
  const navButtons = document.querySelectorAll('.nav-btn');
  const views = document.querySelectorAll('.view');
  const historyList = document.getElementById('historyList');
  const historyDetail = document.getElementById('historyDetail');
  const searchInput = document.getElementById('searchInput');
  const favoriteOnly = document.getElementById('favoriteOnly');
  const refreshHistory = document.getElementById('refreshHistory');
  const toast = document.getElementById('toast');
  const settingsForm = document.getElementById('settingsForm');
  const reloadSettings = document.getElementById('reloadSettings');
  const settingsStatus = document.getElementById('settingsStatus');
  const hotkeyInput = document.getElementById('hotkeyInput');
  const hotkeyHint = document.getElementById('hotkeyHint');
  const saveHotkey = document.getElementById('saveHotkey');

  function showToast(message, type = 'success') {
    toast.textContent = message;
    toast.className = `toast ${type}`;
    requestAnimationFrame(() => {
      toast.classList.remove('hidden');
    });
    setTimeout(() => toast.classList.add('hidden'), 3200);
  }

  function switchView(target) {
    views.forEach((view) => view.classList.remove('active'));
    navButtons.forEach((btn) => btn.classList.remove('active'));
    document.getElementById(target).classList.add('active');
    document.querySelector(`.nav-btn[data-target="${target}"]`).classList.add('active');
  }

  navButtons.forEach((btn) => {
    btn.addEventListener('click', () => switchView(btn.dataset.target));
  });

  async function fetchJSON(url, options = {}) {
    const response = await fetch(url, options);
    if (!response.ok) {
      const text = await response.text();
      throw new Error(text || response.statusText);
    }
    return response.json();
  }

  function renderHistory(items) {
    historyList.innerHTML = '';
    if (!items.length) {
      historyList.innerHTML = '<li class="placeholder">No entries match your filters.</li>';
      historyDetail.innerHTML = '<p class="placeholder">Select an item to preview</p>';
      return;
    }
    items.forEach((item) => {
      const li = document.createElement('li');
      li.className = 'history-item';
      li.innerHTML = `
        <div class="meta"><span>${item.contentType}</span><span>${new Date(item.createdAt).toLocaleString()}</span></div>
        <div class="content-snippet">${escapeHTML(snippet(item.content))}</div>
      `;
      li.addEventListener('click', () => selectHistoryItem(item, li));
      historyList.appendChild(li);
    });
    historyDetail.innerHTML = '<p class="placeholder">Select an item to preview</p>';
  }

  function snippet(text) {
    const trimmed = text.trim();
    return trimmed.length > 140 ? `${trimmed.slice(0, 140)}…` : trimmed || '(empty)';
  }

  function escapeHTML(str) {
    return str.replace(/[&<>"]/g, (c) => ({ '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;' }[c] || c));
  }

  function selectHistoryItem(item, element) {
    document.querySelectorAll('.history-item').forEach((el) => el.classList.remove('active'));
    element.classList.add('active');
    historyDetail.innerHTML = `
      <div class="detail-meta">
        <h3>${escapeHTML(item.contentType)}</h3>
        <p class="muted">Updated ${new Date(item.updatedAt).toLocaleString()} · Favorite: ${item.favorite ? 'Yes' : 'No'}</p>
      </div>
      <pre>${escapeHTML(item.content)}</pre>
      <div class="detail-actions">
        <button id="copyBtn">Copy to Clipboard</button>
        <button id="favoriteBtn" class="secondary">${item.favorite ? 'Unfavorite' : 'Favorite'}</button>
        <button id="deleteBtn" class="danger">Delete</button>
      </div>
    `;
    document.getElementById('copyBtn').addEventListener('click', async () => {
      try {
        await fetchJSON(`/api/history/${item.localId}/select`, { method: 'POST' });
        showToast('Copied to clipboard');
      } catch (err) {
        showToast(err.message, 'error');
      }
    });
    document.getElementById('favoriteBtn').addEventListener('click', async () => {
      try {
        await fetchJSON(`/api/history/${item.localId}/favorite`, {
          method: 'POST',
          headers: { 'Content-Type': 'application/json' },
          body: JSON.stringify({ favorite: !item.favorite }),
        });
        showToast('Favorite updated');
        loadHistory();
      } catch (err) {
        showToast(err.message, 'error');
      }
    });
    document.getElementById('deleteBtn').addEventListener('click', async () => {
      if (!confirm('Delete this clipboard entry?')) return;
      try {
        await fetchJSON(`/api/history/${item.localId}/delete`, { method: 'POST' });
        showToast('Entry deleted');
        loadHistory();
      } catch (err) {
        showToast(err.message, 'error');
      }
    });
  }

  async function loadHistory() {
    try {
      const params = new URLSearchParams();
      if (searchInput.value.trim()) params.set('q', searchInput.value.trim());
      if (favoriteOnly.checked) params.set('favorites', '1');
      const items = await fetchJSON(`/api/history?${params.toString()}`);
      renderHistory(items);
    } catch (err) {
      showToast(err.message, 'error');
    }
  }

  async function loadSettings() {
    try {
      const config = await fetchJSON('/api/config');
      document.getElementById('agentEndpoint').value = config.agentEndpoint || '';
      document.getElementById('serverUrl').value = config.serverUrl || '';
      document.getElementById('userId').value = config.userId || '';
      document.getElementById('deviceId').value = config.deviceId || '';
      document.getElementById('syncInterval').value = config.syncIntervalSeconds || '';
      settingsStatus.textContent = `Next sync interval: ${config.syncIntervalSeconds}s`;
    } catch (err) {
      settingsStatus.textContent = err.message;
    }
  }

  settingsForm.addEventListener('submit', async (event) => {
    event.preventDefault();
    const payload = {
      agentEndpoint: document.getElementById('agentEndpoint').value.trim(),
      serverUrl: document.getElementById('serverUrl').value.trim(),
      userId: document.getElementById('userId').value.trim(),
      deviceId: document.getElementById('deviceId').value.trim(),
      syncIntervalSeconds: Number(document.getElementById('syncInterval').value || '0'),
    };
    try {
      const result = await fetchJSON('/api/config', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(payload),
      });
      settingsStatus.textContent = `Saved. Next sync: ${result.syncIntervalSeconds}s`;
      showToast('Settings saved');
    } catch (err) {
      showToast(err.message, 'error');
    }
  });

  reloadSettings.addEventListener('click', loadSettings);

  async function loadHotkey() {
    const info = await fetchJSON('/api/hotkey');
    hotkeyInput.value = info.combo || '';
    if (!info.supported) {
      hotkeyHint.textContent = info.error || 'Global hotkeys are not available on this system.';
      hotkeyHint.classList.add('error');
      saveHotkey.disabled = true;
    } else {
      hotkeyHint.textContent = 'Example: ctrl+shift+v or cmd+shift+v';
      saveHotkey.disabled = false;
    }
  }

  saveHotkey.addEventListener('click', async () => {
    const combo = hotkeyInput.value.trim();
    if (!combo) {
      showToast('Hotkey cannot be empty', 'error');
      return;
    }
    try {
      const result = await fetchJSON('/api/hotkey', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ combo }),
      });
      showToast(`Hotkey set to ${result.combo}`);
      hotkeyHint.textContent = 'Hotkey updated successfully.';
    } catch (err) {
      showToast(err.message, 'error');
    }
  });

  searchInput.addEventListener('input', () => {
    clearTimeout(searchInput.__timer);
    searchInput.__timer = setTimeout(loadHistory, 250);
  });
  favoriteOnly.addEventListener('change', loadHistory);
  refreshHistory.addEventListener('click', loadHistory);

  loadHistory();
  loadSettings();
  loadHotkey();
})();
