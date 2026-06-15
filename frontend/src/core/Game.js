import { Application } from "pixi.js";
import { Socket } from "./Socket.js";
import { Renderer } from "../rendering/Renderer.js";
import { UIManager } from "../ui/UIManager.js";
import { InputManager } from "./InputManager.js";
import { RoomController } from "./RoomController.js";
import { CONFIG } from "./Config.js";

export class Game {
  constructor() {
    this.app = null;
    this.socket = null;
    this.renderer = null;
    this.ui = null;
    this.input = null;
    this.roomController = null;

    // Current active config (starts with defaults, updated by server)
    this.config = JSON.parse(JSON.stringify(CONFIG));

    this.state = "INITIALIZING"; // INITIALIZING, MENU, CONNECTING, PLAYING, DEAD, ERROR
  }

  async initialize() {
    // Initialize PixiJS Application
    this.app = new Application();
    await this.app.init({
      resizeTo: window,
      backgroundColor: this.config.VISUALS.BACKGROUND_COLOR,
      antialias: true,
      resolution: window.devicePixelRatio || 1,
      autoDensity: true,
    });

    document.getElementById("game-container").appendChild(this.app.canvas);

    // Initialize Managers
    this.ui = new UIManager(this);
    this.renderer = new Renderer(this);
    this.input = new InputManager(this);
    this.socket = new Socket(this);
    this.roomController = new RoomController(this);

    // If a previous session is stored, try to resume it (into the server's
    // grace window); otherwise fall back to the menu.
    await this.resumeSession();

    // Start main loop
    this.app.ticker.add((ticker) => {
      this.update(ticker.deltaTime, ticker.deltaMS);
    });
  }

  // Attempt to resume a stored session after a page reload. If the room is gone
  // or the connection fails, clear the stale session and show the menu.
  async resumeSession() {
    const session = this.socket.loadSession();
    if (!session || !session.playerName || !session.roomID) {
      this.setState("MENU");
      return;
    }

    console.log("Resuming session:", session);
    this.setState("CONNECTING");
    try {
      await this.socket.connect(
        session.playerName,
        session.roomID,
        session.customURL,
      );
      this.setState("PLAYING");
    } catch (err) {
      console.warn("Session resume failed:", err);
      this.socket.clearSession();
      this.setState("MENU");
    }
  }

  // Called when server sends 'init' or config update
  applyServerConfig(serverConfig) {
    if (!serverConfig) return;

    // Map server keys to our internal config structure
    // This handles both camelCase and snake_case from Go backend
    if (serverConfig.WorldSize || serverConfig.world_size) {
      this.config.WORLD.SIZE =
        serverConfig.WorldSize || serverConfig.world_size;
    }

    const physics = this.config.PHYSICS;
    physics.FRICTION =
      serverConfig.Friction || serverConfig.friction || physics.FRICTION;
    physics.ACCELERATION =
      serverConfig.MoveAcceleration ||
      serverConfig.move_acceleration ||
      physics.ACCELERATION;
    physics.MAX_SPEED =
      serverConfig.MaxSpeed || serverConfig.max_speed || physics.MAX_SPEED;

    console.log("Applied server config:", this.config);

    // Notify renderer if world size changed
    if (this.renderer) {
      this.renderer.setupBackground();
    }
  }

  onPlayerDeath() {
    if (this.state === "PLAYING") {
      this.setState("DEAD");
    }
  }

  // Resign: leave the room, drop the stored session, and return to the menu.
  // The server starts a grace period on disconnect, then removes the tank.
  leaveRoom() {
    if (this.state === "MENU" || this.state === "INITIALIZING") return;
    this.socket.clearSession();
    if (this.socket.ws) {
      this.socket.ws.close();
    }
    this.setState("MENU");
  }

  async respawn() {
    const name = this.socket.playerName;
    const room = this.socket.roomID;
    const customURL = this.socket.customURL;

    if (this.socket.ws) {
      this.socket.ws.close();
    }

    await this.connect(name, room, customURL);
  }

  setState(newState) {
    console.log(`Game state: ${this.state} -> ${newState}`);
    this.state = newState;
    this.ui.onStateChange(newState);
  }

  update(deltaTime, deltaMS) {
    if (this.state === "PLAYING") {
      this.renderer.update(deltaTime, deltaMS);
      this.input.update(deltaTime);
    }
  }

  async connect(playerName, roomID = "default", customURL = null) {
    this.setState("CONNECTING");
    try {
      await this.socket.connect(playerName, roomID, customURL);
      this.setState("PLAYING");
    } catch (err) {
      console.error("Connection failed:", err);
      this.setState("ERROR");
      this.ui.showError("Connection failed. Please try again.");
    }
  }
}
