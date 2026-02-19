# Cubby - JavaScript Client

A lightweight JavaScript client for the [Cubby](https://github.com/jasonrdsouza/cubby) object store. Provides a simple state management interface backed by browser `localStorage`, with optional background sync to a Cubby server.

## Features

- **Simple API**: `getState()` / `setState()` for managing a single state object
- **Offline-first**: State is always persisted to `localStorage` for instant access
- **Optional server sync**: Configure a Cubby server to sync state across devices
- **Graceful degradation**: Works with just `localStorage` if no server is configured
- **Background sync**: Debounced sync after changes + periodic interval as a safety net
- **Last-write-wins**: Conflicts between local and remote state are resolved by timestamp
- **Zero dependencies**: Plain JavaScript, no build step required

## Installation

The client is served directly by the Cubby server at `/client.js`:

```html
<script type="module">
  import { Cubby } from 'https://cubby.example.com/client.js';
</script>
```

Alternatively, copy `cubby-client.js` into your project. No package manager or build tools needed.

## Quick Start

### localStorage only (no server)

```html
<script type="module">
  import { Cubby } from './cubby-client.js';

  const cubby = new Cubby({
    key: 'my-app',
    defaultState: { count: 0 },
  });

  await cubby.init();

  console.log(store.getState()); // { count: 0 }

  cubby.setState({ count: 1 });
  // State is saved to localStorage immediately.
  // Survives page reloads.
</script>
```

### With a Cubby server

```html
<script type="module">
  import { Cubby } from './cubby-client.js';

  const cubby = new Cubby({
    key: 'my-app',
    defaultState: { count: 0 },
    server: 'https://cubby.example.com',
    username: 'myuser',
    password: 'mypass',
  });

  cubby.onSyncError((type, err) => {
    console.error('Sync failed:', type, err);
  });

  await cubby.init();
  // On init, the client loads from both localStorage and Cubby,
  // then uses whichever is newer (last-write-wins).

  cubby.setState({ count: 42 });
  // Saved to localStorage immediately.
  // Pushed to Cubby after a 2-second debounce.
</script>
```

### Using as a global (without import)

If you prefer not to use ES module imports, `Cubby` is also available on `window`:

```html
<script type="module" src="cubby-client.js"></script>
<script type="module">
  // Cubby is available globally
  const cubby = new Cubby({ key: 'my-app' });
  await cubby.init();
</script>
```

## API Reference

### `new Cubby(options)`

Creates a new store instance. Does not load any data until `init()` is called.

**Options:**

| Option | Type | Default | Description |
|--------|------|---------|-------------|
| `key` | `string` | *(required)* | Storage key for both localStorage and Cubby |
| `defaultState` | `any` | `{}` | Initial state when no stored state exists |
| `server` | `string` | `null` | Cubby server URL. Omit to use localStorage only |
| `username` | `string` | `null` | Username for Cubby Basic Auth |
| `password` | `string` | `null` | Password for Cubby Basic Auth |
| `persistConfig` | `boolean` | `true` | Save server config to localStorage so it survives page reloads |
| `debounceMs` | `number` | `2000` | Milliseconds to wait after `setState()` before syncing |
| `syncIntervalMs` | `number` | `30000` | Periodic sync interval in milliseconds |

### `cubby.init() → Promise<Cubby>`

Loads state from localStorage and (if a server is configured) from the Cubby server. Resolves conflicts using last-write-wins based on timestamps. Starts the background sync timer if a server is configured.

Returns the store instance for chaining:

```js
const store = await new Cubby({ key: 'app' }).init();
```

### `cubby.getState() → any`

Returns the current state. Throws if `init()` has not been called.

### `cubby.setState(newState)`

Replaces the entire state with `newState`. This is a **full replacement**, not a merge. If you need to update part of the state, read it first:

```js
const state = cubby.getState();
cubby.setState({ ...state, count: state.count + 1 });
```

After calling `setState()`:
1. The new state is written to `localStorage` immediately
2. If a Cubby server is configured, a sync is scheduled after the debounce period

### `cubby.onSyncError(callback)`

Registers a callback that is invoked when a sync operation fails. The callback receives two arguments:

- `type` (string): The error category
- `error` (Error): The error object

Error types:

| Type | Description |
|------|-------------|
| `init-fetch` | Failed to fetch from Cubby during `init()` |
| `sync-push` | Failed to push state to Cubby during background sync |
| `localStorage-save` | Failed to write to localStorage |
| `localStorage-load` | Failed to read from localStorage |

```js
store.onSyncError((type, err) => {
  if (type === 'sync-push') {
    showNotification('Could not save to server. Changes are saved locally.');
  }
});
```

### `cubby.disconnect()`

Disconnects from the Cubby server, clears saved server config from localStorage, and stops background sync. The store continues to work with localStorage only after disconnecting.

### `cubby.destroy()`

Stops all background sync timers and cleans up resources. Attempts one final push to Cubby if there are unsaved changes. Call this when the store is no longer needed.

## Config Persistence

By default, when you provide `server`, `username`, and `password` options, they are saved to localStorage under the key `cubby-config:<your-key>`. On subsequent page loads, you can create the store with just a `key` and the server config will be restored automatically:

```js
// First load: provide server config
const cubby = new Cubby({
  key: 'my-app',
  server: 'https://cubby.example.com',
  username: 'myuser',
  password: 'mypass',
});
await cubby.init(); // config is saved to localStorage

// After page reload: config is restored automatically
const cubby = new Cubby({ key: 'my-app' });
await cubby.init(); // reconnects to Cubby using saved config
```

To stop syncing and clear saved credentials, call `cubby.disconnect()`.

To disable config persistence entirely, set `persistConfig: false`.

**Note:** Credentials are stored in plaintext in localStorage. This is the same security posture as storing API tokens in localStorage (common in SPAs). If your app requires stronger credential security, set `persistConfig: false` and manage credentials yourself.

## How Sync Works

1. **On `init()`**: State is loaded from both localStorage and the Cubby server. The version with the more recent timestamp wins (last-write-wins). If the server is unreachable, the local state is used.

2. **On `setState()`**: The new state is written to localStorage immediately. A sync to Cubby is scheduled after the debounce period (default 2 seconds). Rapid `setState()` calls reset the debounce timer, so only the final state is pushed.

3. **Periodic sync**: Every 30 seconds (configurable), any pending changes are pushed to Cubby. This acts as a safety net in case a debounced sync fails.

4. **Across devices**: When you open the app on another device, `init()` pulls the latest state from Cubby. If the remote state is newer than what's in localStorage, it is used.

## localStorage Format

State is stored in localStorage under the key `cubby:<your-key>` as a JSON string:

```json
{
  "state": { "your": "data" },
  "updatedAt": 1706500000000
}
```

## Example

See `example.html` for a complete working demo (a simple todo list app).

## CORS

If your web app is served from a different origin than the Cubby server, the Cubby server must have CORS enabled. As of recent versions, Cubby includes permissive CORS headers (`Access-Control-Allow-Origin: *`) by default.
