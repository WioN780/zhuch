export class Socket {
  static SESSION_KEY = "zhuch_session";

  constructor(game) {
    this.game = game;
    this.ws = null;
    this.reconnectAttempts = 0;
    this.maxReconnectAttempts = 5;
    // Name-in-use retries are tracked separately: onopen resets reconnectAttempts
    // before the server's rejection arrives, so that counter can't bound them.
    this.nameRetries = 0;
    this.maxNameRetries = 5;
    this.pingInterval = null;

    this.latency = 0;
    this.lastPingTime = 0;

    this._visibilityBound = false;
    this._intentionalClose = false;
  }

  // --- Session persistence (survives page reloads) ---

  saveSession() {
    try {
      localStorage.setItem(
        Socket.SESSION_KEY,
        JSON.stringify({
          playerName: this.playerName,
          roomID: this.roomID,
          customURL: this.customURL ?? null,
        }),
      );
    } catch {
      // localStorage unavailable (private mode / disabled) — resume just won't work.
    }
  }

  loadSession() {
    try {
      const raw = localStorage.getItem(Socket.SESSION_KEY);
      return raw ? JSON.parse(raw) : null;
    } catch {
      return null;
    }
  }

  clearSession() {
    try {
      localStorage.removeItem(Socket.SESSION_KEY);
    } catch {
      // ignore
    }
  }

  async connect(playerName, roomID, customURL = null) {
    this.playerName = playerName;
    this.roomID = roomID;
    this.customURL = customURL;

    this._bindVisibility();

    return new Promise((resolve, reject) => {
      let url;

      if (customURL) {
        url = customURL.startsWith("ws") ? customURL : `ws://${customURL}`;
        if (!url.includes("/ws")) {
          const urlObj = new URL(url.includes("://") ? url : `ws://${url}`);
          if (urlObj.pathname === "/") url += "/ws";
        }
        if (!url.includes("room=")) {
          url += (url.includes("?") ? "&" : "?") + `room=${roomID}`;
        }
      } else {
        // VITE_BACKEND_URL is baked in at build time (set per-Railway-service,
        // not hardcoded here). Falls back to the local dev server.
        const backend = import.meta.env.VITE_BACKEND_URL || "localhost:8080";
        const proto = backend.includes("localhost") ? "ws" : "wss";
        url = `${proto}://${backend}/ws?room=${roomID}`;
      }

      console.log("Connecting to WebSocket:", url);
      this.ws = new WebSocket(url);

      this.ws.onopen = () => {
        console.log("WebSocket connected");
        this.ws.send(playerName);
        this.reconnectAttempts = 0;
        this.startPingLoop();
        // Persist the session so a page reload can transparently resume into
        // the server's grace window (see Game.resumeSession).
        this.saveSession();
        resolve();
      };

      this.ws.onmessage = (event) => {
        const data = JSON.parse(event.data);
        this.handleMessage(data);
      };

      this.ws.onclose = () => {
        console.log("WebSocket disconnected");
        this._clearPingLoop();
        // An error message we already handled (e.g. name-in-use retry) closes
        // the socket too; don't double-schedule a reconnect for it.
        if (this._intentionalClose) {
          this._intentionalClose = false;
          return;
        }
        // Only auto-reconnect when we were actively playing or already retrying.
        // Initial connection failures go through Game.connect()'s catch instead.
        if (this.game.state === "PLAYING" || this.game.state === "RECONNECTING") {
          this._scheduleReconnect();
        }
      };

      this.ws.onerror = (err) => {
        console.error("WebSocket error:", err);
        reject(err);
      };
    });
  }

  startPingLoop() {
    this._clearPingLoop();
    this.pingInterval = setInterval(() => {
      if (this.ws && this.ws.readyState === WebSocket.OPEN) {
        this.lastPingTime = performance.now();
        this.ws.send(JSON.stringify({ type: "ping" }));
      }
    }, 2000);
  }

  _clearPingLoop() {
    if (this.pingInterval) {
      clearInterval(this.pingInterval);
      this.pingInterval = null;
    }
  }

  _bindVisibility() {
    if (this._visibilityBound) return;
    this._visibilityBound = true;
    document.addEventListener("visibilitychange", () => {
      if (!document.hidden && this.game.state === "RECONNECTING") {
        // Tab came back into focus mid-reconnect — restart from attempt 0.
        this.reconnectAttempts = 0;
        this._scheduleReconnect();
      }
    });
  }

  _scheduleReconnect() {
    if (this.reconnectAttempts >= this.maxReconnectAttempts) {
      this.game.setState("ERROR");
      this.game.ui.showError("Connection lost. Please refresh the page.");
      return;
    }
    this.game.setState("RECONNECTING");
    const delay = Math.min(1000 * 2 ** this.reconnectAttempts, 10000);
    this.reconnectAttempts++;
    console.log(`Reconnecting in ${delay}ms (attempt ${this.reconnectAttempts})`);
    setTimeout(() => this._doReconnect(), delay);
  }

  async _doReconnect() {
    if (this.game.state !== "RECONNECTING") return;
    try {
      await this.connect(this.playerName, this.roomID, this.customURL);
      this.game.setState("PLAYING");
    } catch {
      this._scheduleReconnect();
    }
  }

  handleMessage(data) {
    if (data.type === "pong") {
      this.latency = (performance.now() - this.lastPingTime) / 2;
      return;
    }

    if (data.type === "dead") {
      this.game.onPlayerDeath();
      return;
    }

    if (data.type === "error") {
      console.error("Server error:", data.message);
      // Fast-reload race: the previous connection's name may not be released
      // yet when we reconnect into the grace window. Retry via backoff rather
      // than dropping the player to the menu.
      const retryable = /in use/i.test(data.message || "");
      const playing =
        this.game.state === "PLAYING" || this.game.state === "RECONNECTING";
      if (retryable && playing && this.nameRetries < this.maxNameRetries) {
        this.nameRetries++;
        this._intentionalClose = true; // server will close after this error
        this._scheduleReconnect();
        return;
      }
      this.nameRetries = 0;
      this.clearSession();
      this.game.setState("MENU");
      this.game.ui.showError(data.message);
      return;
    }

    if (data.type === "init") {
      // Real success: the server accepted us into the room.
      this.nameRetries = 0;
      this.game.renderer.setPlayerID(data.tank_id);
      this.game.applyServerConfig(data.config);
      if (Array.isArray(data.obstacles)) {
        this.game.renderer.setObstacles(data.obstacles);
      }
      return;
    }

    if (data.entities && Array.isArray(data.entities)) {
      this.game.renderer.processStateUpdate(data.entities, data.metrics);
    } else if (Array.isArray(data)) {
      this.game.renderer.processStateUpdate(data);
    }
  }

  sendInput(input) {
    if (this.ws && this.ws.readyState === WebSocket.OPEN) {
      this.ws.send(JSON.stringify({ type: "input", ...input }));
    }
  }
}
