export class Socket {
  constructor(game) {
    this.game = game;
    this.ws = null;
    this.reconnectAttempts = 0;
    this.maxReconnectAttempts = 5;
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
        // Handshake: Send player name
        this.ws.send(playerName);
        this.reconnectAttempts = 0;
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

  handleMessage(data) {
    if (data.type === "init") {
      this.game.renderer.setPlayerID(data.tank_id);
      return;
    }

    // Default: it's a game state update (array of entities)
    if (Array.isArray(data)) {
      this.game.renderer.processStateUpdate(data);
    }
  }

  sendInput(input) {
    if (this.ws && this.ws.readyState === WebSocket.OPEN) {
      this.ws.send(
        JSON.stringify({
          type: "input",
          ...input,
        }),
      );
    }
  }
}
