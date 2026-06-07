export class Socket {
  constructor(game) {
    this.game = game;
    this.ws = null;
    this.reconnectAttempts = 0;
    this.maxReconnectAttempts = 5;

    this.latency = 0;
    this.lastPingTime = 0;
  }

  async connect(playerName, roomID, customURL = null) {
    this.playerName = playerName;
    this.roomID = roomID;
    this.customURL = customURL;

    return new Promise((resolve, reject) => {
      let url;

      if (customURL) {
        // Handle custom URL entry
        url = customURL.startsWith("ws") ? customURL : `ws://${customURL}`;
        if (!url.includes("/ws")) {
          const urlObj = new URL(url.includes("://") ? url : `ws://${url}`);
          if (urlObj.pathname === "/") url += "/ws";
        }
        // Ensure roomID is attached
        if (!url.includes("room=")) {
          url += (url.includes("?") ? "&" : "?") + `room=${roomID}`;
        }
      } else {
        // Default to Production Railway Server
        // Always use wss for production
        url = `wss://zhuch-production.up.railway.app/ws?room=${roomID}`;
      }

      console.log("Connecting to WebSocket:", url);
      this.ws = new WebSocket(url);

      this.ws.onopen = () => {
        console.log("WebSocket connected");
        this.ws.send(playerName);
        this.reconnectAttempts = 0;
        this.startPingLoop();
        resolve();
      };

      this.ws.onmessage = (event) => {
        const data = JSON.parse(event.data);
        this.handleMessage(data);
      };

      this.ws.onclose = () => {
        console.log("WebSocket disconnected");
        if (this.game.state === "PLAYING") {
          this.game.setState("ERROR");
          this.game.ui.showError("Disconnected from server.");
        }
      };

      this.ws.onerror = (err) => {
        console.error("WebSocket error:", err);
        reject(err);
      };
    });
  }

  startPingLoop() {
    setInterval(() => {
      if (this.ws && this.ws.readyState === WebSocket.OPEN) {
        this.lastPingTime = performance.now();
        this.ws.send(JSON.stringify({ type: "ping" }));
      }
    }, 2000);
  }

  handleMessage(data) {
    if (data.type === "pong") {
      // Calculate round-trip time and store it as one-way latency for the HUD
      const rtt = performance.now() - this.lastPingTime;
      this.latency = rtt / 2;
      return;
    }

    if (data.type === "dead") {
      this.game.onPlayerDeath();
      return;
    }

    if (data.type === "error") {
      console.error("Server error:", data.message);
      this.game.setState("MENU");
      this.game.ui.showError(data.message);
      return;
    }

    if (data.type === "init") {
      this.game.renderer.setPlayerID(data.tank_id);
      this.game.applyServerConfig(data.config);
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
