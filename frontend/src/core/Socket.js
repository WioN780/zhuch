export class Socket {
  constructor(game) {
    this.game = game;
    this.ws = null;
    this.reconnectAttempts = 0;
    this.maxReconnectAttempts = 5;

    this.latency = 0;
    this.lastPingTime = 0;
  }

  async connect(playerName, roomID) {
    return new Promise((resolve, reject) => {
      const isLocal =
        window.location.hostname === "localhost" ||
        window.location.hostname === "127.0.0.1";
      const protocol = isLocal ? "ws:" : "wss:";
      const backendHost = isLocal
        ? "localhost:8080"
        : "zhuch-production.up.railway.app";
      const url = `${protocol}//${backendHost}/ws?room=${roomID}`;

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
    // Simple ping loop if the backend supports it (or just for timing)
    setInterval(() => {
      if (this.ws && this.ws.readyState === WebSocket.OPEN) {
        this.lastPingTime = performance.now();
        this.ws.send(JSON.stringify({ type: "ping" }));
      }
    }, 2000);
  }

  handleMessage(data) {
    if (data.type === "pong") {
      this.latency = (performance.now() - this.lastPingTime) / 2;
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
      this.game.config = data.config; // Store server config for prediction parity
      console.log("Game config received:", this.game.config);
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
