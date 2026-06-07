import { Container, Graphics } from "pixi.js";

export class EntityBase {
  constructor(id, manager) {
    this.id = id;
    this.manager = manager;
    this.container = new Container();

    this.position = { x: 0, y: 0 };
    this.velocity = { x: 0, y: 0 };
    this.rotation = 0;

    // Visual Smoothing (Hides snaps from server updates)
    this.visualOffset = { x: 0, y: 0 };
    this.offsetBleed = 0.15;

    const config = manager.renderer.game.config;

    this.stateBuffer = [];
    this.interpolationDelay = config.SMOOTHING.TANK_INTERPOLATION_DELAY;

    this.initialized = false;
    this.isBullet = false;
    this.lastHealthPct = -1;

    this.healthBar = new Container();
    this.healthBarBg = new Graphics();
    this.healthBarFill = new Graphics();
    this.healthBar.addChild(this.healthBarBg);
    this.healthBar.addChild(this.healthBarFill);
    this.container.addChild(this.healthBar);
    this.healthBar.visible = false;
  }

  get isLocal() {
    return this.id === this.manager?.renderer?.playerID;
  }

  updateData(data, timestamp = performance.now()) {
    const object = data.object || data.Object;
    const vel = data.vel || data.Vel;
    const center = object?.Center || object?.center || { X: 0, Y: 0 };
    const rotation =
      data.orientation !== undefined ? data.orientation : data.Orientation || 0;

    const newState = {
      timestamp,
      pos: { x: center.X || center.x, y: center.Y || center.y },
      vel: { x: vel?.X || vel?.x || 0, y: vel?.Y || vel?.y || 0 },
      rotation: rotation,
      health: data.health !== undefined ? data.health : data.Health,
      maxHealth:
        data.max_health !== undefined ? data.max_health : data.MaxHealth || 100,
      name: data.name || data.Name,
      ownerID: data.owner_id || data.OwnerID,
    };

    if (!this.initialized) {
      const config = this.manager.renderer.game.config;
      this.isBullet = newState.ownerID !== undefined;

      // Bullets stay on server-time (0 delay) for maximum "authority"
      // Remote tanks stay on a delay for smooth interpolation
      this.interpolationDelay = this.isBullet
        ? 0
        : config.SMOOTHING.TANK_INTERPOLATION_DELAY;

      this.position = { ...newState.pos };
      this.velocity = { ...newState.vel };
      this.rotation = newState.rotation;

      // Set timestamp to 'now' so that interpolationDelay-based rendering
      // initially treats this as a 'future' state and extrapolates forward
      // from it, preventing the warp-back.
      this.stateBuffer.push({
        ...newState,
        timestamp: performance.now(),
      });

      this.initialized = true;
    }

    // Capture visual offset for smoothing the correction if we are on server-time (bullets)
    if (this.isBullet && !this.isLocal) {
      this.visualOffset.x = this.position.x - newState.pos.x;
      this.visualOffset.y = this.position.y - newState.pos.y;
      this.position = { ...newState.pos };
    }

    // Don't add to buffer if it's the local player (they use prediction)
    if (!this.isLocal) {
      this.stateBuffer.push(newState);
      if (this.stateBuffer.length > 20) this.stateBuffer.shift();
    }

    if (this.isLocal) {
      this.onServerUpdate(newState);
    }

    this.health = newState.health;
    this.maxHealth = newState.maxHealth;

    // Detect Damage
    if (this.oldHealth !== undefined && this.health < this.oldHealth) {
      this.triggerHitEffect();
      if (this.isLocal) {
        this.manager.renderer.shake(10);

        // Check for death
        if (this.health <= 0) {
          setTimeout(() => {
            this.manager.renderer.game.setState("MENU");
          }, 1500); // Wait a bit for the explosion effect
        }
      }
    }
    this.oldHealth = this.health;

    this.velocity = { ...newState.vel }; // Store latest velocity for extrapolation
    this.updateHealthBar();
  }

  triggerHitEffect() {
    // Simple flash effect
    this.container.alpha = 0.5;
    setTimeout(() => {
      if (this.container) this.container.alpha = 1.0;
    }, 50);
  }

  onServerUpdate(serverState) {}

  update(deltaTime, deltaMS) {
    if (!this.initialized) return;

    if (this.isLocal) {
      this.updateLocal(deltaTime, deltaMS);
    } else {
      this.updateInterpolation(deltaMS, deltaTime);
    }

    // Sync visuals
    this.syncContainer(deltaTime);
  }

  syncContainer(deltaTime) {
    // Apply visual offset smoothing (bleed)
    // Frame-rate independent bleed
    const bleed = 1 - Math.pow(1 - this.offsetBleed, deltaTime || 1);
    this.visualOffset.x *= 1 - bleed;
    this.visualOffset.y *= 1 - bleed;

    if (Math.abs(this.visualOffset.x) < 0.01) this.visualOffset.x = 0;
    if (Math.abs(this.visualOffset.y) < 0.01) this.visualOffset.y = 0;

    this.container.position.set(
      this.position.x + this.visualOffset.x,
      this.position.y + this.visualOffset.y,
    );
    this.syncRotation();
  }

  syncRotation() {
    this.container.rotation = this.rotation;
  }

  updateInterpolation(deltaMS, deltaTime) {
    if (this.stateBuffer.length === 0) return;

    const renderTime = performance.now() - this.interpolationDelay;

    // If we're ahead of the oldest state but behind the newest, interpolate
    if (
      this.stateBuffer.length >= 2 &&
      renderTime >= this.stateBuffer[0].timestamp &&
      this.interpolationDelay > 0
    ) {
      let i = 0;
      for (; i < this.stateBuffer.length - 1; i++) {
        if (this.stateBuffer[i + 1].timestamp > renderTime) break;
      }

      const stateA = this.stateBuffer[i];
      const stateB = this.stateBuffer[i + 1];

      if (stateA && stateB && renderTime <= stateB.timestamp) {
        const lerpFactor =
          (renderTime - stateA.timestamp) /
          (stateB.timestamp - stateA.timestamp);

        this.position.x =
          stateA.pos.x + (stateB.pos.x - stateA.pos.x) * lerpFactor;
        this.position.y =
          stateA.pos.y + (stateB.pos.y - stateA.pos.y) * lerpFactor;

        let diff = stateB.rotation - stateA.rotation;
        while (diff > Math.PI) diff -= Math.PI * 2;
        while (diff < -Math.PI) diff += Math.PI * 2;
        this.rotation = stateA.rotation + diff * lerpFactor;
        return;
      }
    }

    // BUFFER UNDERRUN or ZERO-DELAY (Bullets): Extrapolate using last known velocity
    const latest = this.stateBuffer[this.stateBuffer.length - 1];
    const config = this.manager.renderer.game.config;

    // If it's a very fresh state and we have a delay, just snap
    if (
      Math.abs(renderTime - latest.timestamp) < 16 &&
      this.interpolationDelay > 0
    ) {
      this.position.x = latest.pos.x;
      this.position.y = latest.pos.y;
      this.rotation = latest.rotation;
    } else {
      // Extrapolate forward
      const ratio = deltaMS / config.PHYSICS.SERVER_TICK_MS;
      this.position.x += this.velocity.x * ratio;
      this.position.y += this.velocity.y * ratio;
      this.rotation = latest.rotation;
    }
  }

  updateLocal(deltaTime, deltaMS) {}

  updateHealthBar() {
    const config = this.manager.renderer.game.config.VISUALS.HEALTH_BAR;
    if (this.health === undefined || this.health >= this.maxHealth) {
      this.healthBar.visible = false;
      return;
    }
    const pct = Math.max(0, this.health / this.maxHealth);
    if (this.lastHealthPct === pct) return;
    this.lastHealthPct = pct;
    this.healthBar.visible = true;

    const { WIDTH, HEIGHT, OFFSET_Y } = config;

    this.healthBarBg
      .clear()
      .rect(-WIDTH / 2, OFFSET_Y, WIDTH, HEIGHT)
      .fill({ color: 0x000000, alpha: 0.5 });
    this.healthBarFill
      .clear()
      .rect(-WIDTH / 2, OFFSET_Y, WIDTH * pct, HEIGHT)
      .fill({ color: 0x81c784 });
  }

  reset() {
    this.initialized = false;
    this.stateBuffer = [];
    this.lastHealthPct = -1;
    this.healthBar.visible = false;
  }

  destroy() {
    if (this.container.parent)
      this.container.parent.removeChild(this.container);
    this.container.destroy({ children: true });
  }
}
