/**
 * Cubby - A JavaScript client for the Cubby object store.
 *
 * Provides a simple state management interface backed by browser localStorage,
 * with optional background sync to a Cubby server. Works offline with just
 * localStorage if no server is configured.
 */
class Cubby {
  /**
   * Create a new Cubby.
   *
   * @param {Object} options
   * @param {string} options.key - Storage key (used for both localStorage and Cubby)
   * @param {*} [options.defaultState={}] - Initial state when no stored state exists
   * @param {string} [options.server] - Cubby server URL (omit to use localStorage only)
   * @param {string} [options.username] - Username for Cubby Basic Auth
   * @param {string} [options.password] - Password for Cubby Basic Auth
   * @param {boolean} [options.persistConfig=true] - Save server config to localStorage for reuse across page loads
   * @param {number} [options.debounceMs=2000] - Delay before syncing after setState
   * @param {number} [options.syncIntervalMs=30000] - Periodic sync interval in ms
   */
  constructor(options) {
    if (!options || !options.key) {
      throw new Error('Cubby requires a "key" option');
    }

    this.key = options.key;
    this.defaultState = options.defaultState !== undefined ? options.defaultState : {};
    this.persistConfig = options.persistConfig !== undefined ? options.persistConfig : true;
    this.debounceMs = options.debounceMs !== undefined ? options.debounceMs : 2000;
    this.syncIntervalMs = options.syncIntervalMs !== undefined ? options.syncIntervalMs : 30000;

    // Use provided server config, or restore from localStorage
    if (options.server) {
      this.server = options.server;
      this.username = options.username || null;
      this.password = options.password || null;
      if (this.persistConfig) {
        this._saveConfig();
      }
    } else {
      const saved = this.persistConfig ? this._loadConfig() : null;
      if (saved) {
        this.server = saved.server;
        this.username = saved.username;
        this.password = saved.password;
      } else {
        this.server = null;
        this.username = null;
        this.password = null;
      }
    }

    this._state = null;
    this._updatedAt = null;
    this._debounceTimer = null;
    this._syncInterval = null;
    this._syncErrorCallback = null;
    this._dirty = false;
    this._initialized = false;
  }

  /**
   * Initialize the store by loading state from localStorage and (if configured)
   * fetching from the Cubby server. Uses last-write-wins to resolve conflicts.
   * Must be called before getState() or setState().
   *
   * @returns {Promise<Cubby>} This store instance (for chaining)
   */
  async init() {
    const local = this._loadFromLocalStorage();

    if (!this._cubbyConfigured()) {
      if (local) {
        this._state = local.state;
        this._updatedAt = local.updatedAt;
      } else {
        this._state = this.defaultState;
        this._updatedAt = Date.now();
        this._saveToLocalStorage();
      }
      this._initialized = true;
      return this;
    }

    // Fetch from Cubby server
    let remote = null;
    try {
      remote = await this._fetchFromCubby();
    } catch (err) {
      this._reportError('init-fetch', err);
    }

    // Resolve local vs remote with last-write-wins
    if (local && remote) {
      if (remote.updatedAt > local.updatedAt) {
        this._state = remote.state;
        this._updatedAt = remote.updatedAt;
        this._saveToLocalStorage();
      } else {
        this._state = local.state;
        this._updatedAt = local.updatedAt;
        this._dirty = true;
      }
    } else if (remote) {
      this._state = remote.state;
      this._updatedAt = remote.updatedAt;
      this._saveToLocalStorage();
    } else if (local) {
      this._state = local.state;
      this._updatedAt = local.updatedAt;
      this._dirty = true;
    } else {
      this._state = this.defaultState;
      this._updatedAt = Date.now();
      this._saveToLocalStorage();
      this._dirty = true;
    }

    this._startPeriodicSync();
    this._initialized = true;

    // If local state was newer, push it to Cubby right away
    if (this._dirty) {
      this._scheduleDebouncedSync();
    }

    return this;
  }

  /**
   * Get the current state.
   *
   * @returns {*} The current state object
   */
  getState() {
    if (!this._initialized) {
      throw new Error('Cubby not initialized. Call init() first.');
    }
    return this._state;
  }

  /**
   * Replace the current state. Saves to localStorage immediately and
   * schedules a background sync to Cubby (if configured).
   *
   * @param {*} newState - The new state (full replacement, not merged)
   */
  setState(newState) {
    if (!this._initialized) {
      throw new Error('Cubby not initialized. Call init() first.');
    }
    this._state = newState;
    this._updatedAt = Date.now();
    this._saveToLocalStorage();

    if (this._cubbyConfigured()) {
      this._dirty = true;
      this._scheduleDebouncedSync();
    }
  }

  /**
   * Register a callback for sync errors. The callback receives the error
   * type (string) and the Error object.
   *
   * Error types:
   * - "init-fetch": Failed to fetch from Cubby during init()
   * - "sync-push": Failed to push state to Cubby during background sync
   * - "localStorage-save": Failed to save to localStorage
   * - "localStorage-load": Failed to load from localStorage
   *
   * @param {Function} callback - Called as callback(errorType, error)
   */
  onSyncError(callback) {
    this._syncErrorCallback = callback;
  }

  /**
   * Disconnect from the Cubby server and clear saved config from localStorage.
   * The store continues to work with localStorage only after disconnecting.
   */
  disconnect() {
    this.destroy();
    this.server = null;
    this.username = null;
    this.password = null;
    this._clearConfig();
  }

  /**
   * Clean up timers and stop background sync. Attempts one final push
   * to Cubby if there are unsaved changes.
   */
  destroy() {
    if (this._debounceTimer) {
      clearTimeout(this._debounceTimer);
      this._debounceTimer = null;
    }
    if (this._syncInterval) {
      clearInterval(this._syncInterval);
      this._syncInterval = null;
    }
    if (this._dirty && this._cubbyConfigured()) {
      this._pushToCubby().catch(err => this._reportError('sync-push', err));
    }
    this._initialized = false;
  }

  // --- Private methods ---

  _cubbyConfigured() {
    return this.server !== null;
  }

  _localStorageKey() {
    return 'cubby:' + this.key;
  }

  _configKey() {
    return 'cubby-config:' + this.key;
  }

  _saveConfig() {
    try {
      const config = JSON.stringify({
        server: this.server,
        username: this.username,
        password: this.password,
      });
      localStorage.setItem(this._configKey(), config);
    } catch (err) {
      this._reportError('localStorage-save', err);
    }
  }

  _loadConfig() {
    try {
      const raw = localStorage.getItem(this._configKey());
      if (!raw) return null;
      const config = JSON.parse(raw);
      if (config && config.server) {
        return config;
      }
      return null;
    } catch (err) {
      this._reportError('localStorage-load', err);
      return null;
    }
  }

  _clearConfig() {
    try {
      localStorage.removeItem(this._configKey());
    } catch (err) {
      this._reportError('localStorage-save', err);
    }
  }

  _saveToLocalStorage() {
    try {
      const data = JSON.stringify({
        state: this._state,
        updatedAt: this._updatedAt,
      });
      localStorage.setItem(this._localStorageKey(), data);
    } catch (err) {
      this._reportError('localStorage-save', err);
    }
  }

  _loadFromLocalStorage() {
    try {
      const raw = localStorage.getItem(this._localStorageKey());
      if (!raw) return null;
      const data = JSON.parse(raw);
      if (data && data.state !== undefined && data.updatedAt) {
        return { state: data.state, updatedAt: data.updatedAt };
      }
      return null;
    } catch (err) {
      this._reportError('localStorage-load', err);
      return null;
    }
  }

  async _fetchFromCubby() {
    const url = this._cubbyUrl();
    const headers = {};
    if (this.username && this.password) {
      headers['Authorization'] = 'Basic ' + btoa(this.username + ':' + this.password);
    }

    const response = await fetch(url, { headers });

    if (response.status === 404) {
      return null;
    }

    if (!response.ok) {
      throw new Error('Cubby fetch failed: ' + response.status + ' ' + response.statusText);
    }

    const lastModified = response.headers.get('Last-Modified');
    const updatedAt = lastModified ? new Date(lastModified).getTime() : 0;

    const text = await response.text();
    let state;
    try {
      state = JSON.parse(text);
    } catch (e) {
      // Stored data is not valid JSON; treat as no remote state
      return null;
    }

    return { state, updatedAt };
  }

  async _pushToCubby() {
    const url = this._cubbyUrl();
    const headers = {
      'Content-Type': 'application/json',
    };
    if (this.username && this.password) {
      headers['Authorization'] = 'Basic ' + btoa(this.username + ':' + this.password);
    }

    const response = await fetch(url, {
      method: 'POST',
      headers,
      body: JSON.stringify(this._state),
    });

    if (!response.ok) {
      throw new Error('Cubby push failed: ' + response.status + ' ' + response.statusText);
    }

    this._dirty = false;
  }

  _cubbyUrl() {
    const base = this.server.replace(/\/+$/, '');
    return base + '/' + encodeURIComponent(this.key);
  }

  _scheduleDebouncedSync() {
    if (this._debounceTimer) {
      clearTimeout(this._debounceTimer);
    }
    this._debounceTimer = setTimeout(() => {
      this._debounceTimer = null;
      this._syncNow();
    }, this.debounceMs);
  }

  _startPeriodicSync() {
    if (this._syncInterval) {
      clearInterval(this._syncInterval);
    }
    this._syncInterval = setInterval(() => {
      this._syncNow();
    }, this.syncIntervalMs);
  }

  async _syncNow() {
    if (!this._dirty || !this._cubbyConfigured()) return;
    try {
      await this._pushToCubby();
    } catch (err) {
      this._reportError('sync-push', err);
    }
  }

  _reportError(type, error) {
    console.warn('Cubby [' + type + ']:', error);
    if (this._syncErrorCallback) {
      try {
        this._syncErrorCallback(type, error);
      } catch (e) {
        console.warn('Cubby: error in syncError callback:', e);
      }
    }
  }
}

// Make available as a browser global
if (typeof window !== 'undefined') {
  window.Cubby = Cubby;
}

// ES module export
export { Cubby };
export default Cubby;
