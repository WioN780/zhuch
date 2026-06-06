export class InputManager {
  constructor(game) {
    this.game = game;
    this.keys = new Set();
    this.mousePos = { x: 0, y: 0 };
    this.isMouseDown = false;

    // Throttling for network efficiency
    this.lastSentInput = null;
    this.lastSendTimestamp = 0;
    this.sendInterval = 50; // 20Hz (matches server)

    window.addEventListener("keydown", (e) => this.keys.add(e.code));
    window.addEventListener("keyup", (e) => this.keys.delete(e.code));
    window.addEventListener("mousemove", (e) => {
      this.mousePos.x = e.clientX;
      this.mousePos.y = e.clientY;
    });
    window.addEventListener("mousedown", () => (this.isMouseDown = true));
    window.addEventListener("mouseup", () => (this.isMouseDown = false));
    window.addEventListener(
      "wheel",
      (e) => {
        const zoomDelta = -e.deltaY * 0.001;
        this.game.renderer.camera.adjustZoom(zoomDelta);
      },
      { passive: false },
    );
  }

  update(deltaTime) {
    if (this.game.state !== "PLAYING") return;

    const inputVector = { x: 0, y: 0 };
    if (this.keys.has("KeyW") || this.keys.has("ArrowUp")) inputVector.y -= 1;
    if (this.keys.has("KeyS") || this.keys.has("ArrowDown")) inputVector.y += 1;
    if (this.keys.has("KeyA") || this.keys.has("ArrowLeft")) inputVector.x -= 1;
    if (this.keys.has("KeyD") || this.keys.has("ArrowRight"))
      inputVector.x += 1;

    // Normalize input vector
    const length = Math.sqrt(
      inputVector.x * inputVector.x + inputVector.y * inputVector.y,
    );
    if (length > 0) {
      inputVector.x /= length;
      inputVector.y /= length;
    }

    // Get player tank for current orientation
    const playerTank = this.game.renderer.entityManager.getEntity(
      this.game.renderer.playerID,
    );
    let orientation = 0;

    if (playerTank) {
      // Local visual rotation for instant feedback
      const worldMousePos = this.game.renderer.screenToWorld(
        this.mousePos.x,
        this.mousePos.y,
      );
      const dx = worldMousePos.x - playerTank.position.x;
      const dy = worldMousePos.y - playerTank.position.y;
      playerTank.targetBarrelAngle = Math.atan2(dy, dx);
      orientation = playerTank.barrelAngle;
    }

    // THROTTLED INPUT SENDING
    const now = performance.now();
    const currentInput = {
      type: this.isMouseDown ? "fire" : "input",
      input_vector: inputVector,
      orientation: orientation,
    };

    // Only send if time has passed OR if a critical action changed (firing)
    const timeToUpdate = now - this.lastSendTimestamp >= this.sendInterval;
    const actionChanged = this.lastSentInput?.type !== currentInput.type;

    if (timeToUpdate || actionChanged) {
      this.game.socket.sendInput(currentInput);
      this.lastSentInput = JSON.parse(JSON.stringify(currentInput));
      this.lastSendTimestamp = now;
    }
  }
}
