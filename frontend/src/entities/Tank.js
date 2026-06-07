import { Graphics, Text, TextStyle } from "pixi.js";
import { EntityBase } from "./EntityBase.js";

export class Tank extends EntityBase {
  constructor(id, manager) {
    super(id, manager);

    this.body = new Graphics();
    this.barrel = new Graphics();

    // Reconciliation state
    this.serverPosition = { x: 0, y: 0 };
    this.isServerPosSet = false;

    const config = manager.renderer.game.config;

    // Visual Smoothing (Separate logical pos from rendered pos)
    this.visualOffset = { x: 0, y: 0 };
    this.offsetBleed = config.SMOOTHING.OFFSET_BLEED;

    // Configuration Parity (Initialized from game config)
    this.friction = config.PHYSICS.FRICTION;
    this.acceleration = config.PHYSICS.ACCELERATION;
    this.maxSpeed = config.PHYSICS.MAX_SPEED;
    this.weight = config.PHYSICS.WEIGHT;

    // UI
    const style = new TextStyle({
      fontFamily: "Inter, sans-serif",
      fontSize: 14,
      fill: "#ffffff",
      fontWeight: "500",
      dropShadow: { alpha: 0.5, blur: 4, color: "#000000", distance: 2 },
    });
    this.nameTag = new Text({ text: "", style });
    this.nameTag.anchor.set(0.5, 1);
    this.nameTag.position.set(0, -30);

    this.container.addChild(this.barrel);
    this.container.addChild(this.body);
    this.container.addChild(this.nameTag);

    this.radius = config.VISUALS.TANK_RADIUS;
    this.barrelAngle = 0;
    this.targetBarrelAngle = 0;
    this.barrelRecoil = 0;

    this.draw();
  }

  draw() {
    this.body
      .clear()
      .circle(0, 0, this.radius)
      .fill({ color: 0x1a1a1a })
      .stroke({ color: 0xffffff, width: 1.5, alpha: 0.8 });

    this.barrel
      .clear()
      .rect(0, -this.radius * 0.4, this.radius * 1.5, this.radius * 0.8)
      .fill({ color: 0x1a1a1a })
      .stroke({ color: 0xffffff, width: 1.5, alpha: 0.8 });
  }

  // Authoritative update from server
  onServerUpdate(serverState) {
    if (!this.isLocal) return;

    // 1. If it's the first update, snap logic and visuals
    if (!this.isServerPosSet) {
      this.position.x = serverState.pos.x;
      this.position.y = serverState.pos.y;
      this.serverPosition = { ...serverState.pos };
      this.isServerPosSet = true;
      return;
    }

    // 2. Calculate the "Prediction Error"
    const dx = serverState.pos.x - this.position.x;
    const dy = serverState.pos.y - this.position.y;
    const distSq = dx * dx + dy * dy;

    if (distSq > 2500) {
      this.position.x = serverState.pos.x;
      this.position.y = serverState.pos.y;
      this.visualOffset = { x: 0, y: 0 };
    } else if (distSq > 1) {
      this.visualOffset.x = this.position.x - serverState.pos.x;
      this.visualOffset.y = this.position.y - serverState.pos.y;

      this.position.x = serverState.pos.x;
      this.position.y = serverState.pos.y;
    }

    this.serverPosition = { ...serverState.pos };
    this.velocity = { ...serverState.vel };
  }

  // Instant local response
  updateLocal(deltaTime, deltaMS) {
    const input = this.manager.renderer.game.input;
    if (!input) return;

    const inputVector = { x: 0, y: 0 };
    if (input.keys.has("KeyW") || input.keys.has("ArrowUp")) inputVector.y -= 1;
    if (input.keys.has("KeyS") || input.keys.has("ArrowDown"))
      inputVector.y += 1;
    if (input.keys.has("KeyA") || input.keys.has("ArrowLeft"))
      inputVector.x -= 1;
    if (input.keys.has("KeyD") || input.keys.has("ArrowRight"))
      inputVector.x += 1;

    // Normalize
    const len = Math.sqrt(
      inputVector.x * inputVector.x + inputVector.y * inputVector.y,
    );
    if (len > 0) {
      inputVector.x /= len;
      inputVector.y /= len;
    }

    // Physics Parity: Use time-scaled ratios to match server tick rate
    const ratio =
      deltaMS / this.manager.renderer.game.config.PHYSICS.SERVER_TICK_MS;

    // 1. Acceleration
    this.velocity.x += inputVector.x * this.acceleration * ratio;
    this.velocity.y += inputVector.y * this.acceleration * ratio;

    // 2. Movement
    this.position.x += this.velocity.x * ratio;
    this.position.y += this.velocity.y * ratio;

    // 3. Friction (Exponential)
    const frameFriction = Math.pow(this.friction, ratio);
    this.velocity.x *= frameFriction;
    this.velocity.y *= frameFriction;

    // 4. Speed Cap
    const speed = Math.sqrt(
      this.velocity.x * this.velocity.x + this.velocity.y * this.velocity.y,
    );
    if (speed > this.maxSpeed) {
      this.velocity.x = (this.velocity.x / speed) * this.maxSpeed;
      this.velocity.y = (this.velocity.y / speed) * this.maxSpeed;
    }

    // Barrel
    const worldMousePos = this.manager.renderer.screenToWorld(
      input.mousePos.x,
      input.mousePos.y,
    );
    this.targetBarrelAngle = Math.atan2(
      worldMousePos.y - this.position.y,
      worldMousePos.x - this.position.x,
    );
  }

  triggerRecoil() {
    this.barrelRecoil = 10;
    if (this.isLocal) {
      this.manager.renderer.shake(3);
    }
  }

  update(deltaTime, deltaMS) {
    super.update(deltaTime, deltaMS);

    // Smooth rotation
    let diff = this.targetBarrelAngle - this.barrelAngle;
    while (diff > Math.PI) diff -= Math.PI * 2;
    while (diff < -Math.PI) diff += Math.PI * 2;
    this.barrelAngle += diff * 0.2 * deltaTime; // Slightly slower for smoothness
    this.barrel.rotation = this.barrelAngle;

    // Recoil smoothing
    if (this.barrelRecoil > 0) {
      this.barrelRecoil *= Math.pow(0.8, deltaTime);
      if (this.barrelRecoil < 0.1) this.barrelRecoil = 0;
    }
    this.barrel.position.set(
      Math.cos(this.barrelAngle) * -this.barrelRecoil,
      Math.sin(this.barrelAngle) * -this.barrelRecoil,
    );

    // For local player, we apply the visual offset to hide reconciliation snaps
    if (this.isLocal) {
      this.container.position.set(
        this.position.x + this.visualOffset.x,
        this.position.y + this.visualOffset.y,
      );

      // Gradually melt the visual offset (smoother bleed)
      const bleed = 1 - Math.pow(1 - this.offsetBleed, deltaTime);
      this.visualOffset.x *= 1 - bleed;
      this.visualOffset.y *= 1 - bleed;

      if (Math.abs(this.visualOffset.x) < 0.01) this.visualOffset.x = 0;
      if (Math.abs(this.visualOffset.y) < 0.01) this.visualOffset.y = 0;
    } else {
      // For non-local tanks, syncContainer in EntityBase handles it,
      // but we need to ensure barrel rotation is updated if it's not part of base rotation
      this.barrel.rotation = this.rotation;
    }
  }

  updateData(data) {
    super.updateData(data);

    // Sync dynamic stats
    const maxSpeed = data.max_speed || data.MaxSpeed;
    const accel = data.move_acceleration || data.MoveAcceleration;
    const friction = data.friction || data.Friction;
    const weight = data.weight || data.Weight;

    if (maxSpeed) this.maxSpeed = maxSpeed;
    if (accel) this.acceleration = accel;
    if (friction) this.friction = friction;
    if (weight) this.weight = weight;

    const name = data.name || data.Name;
    if (name && this.nameTag.text !== name) this.nameTag.text = name;

    const object = data.object || data.Object;
    if (object && (object.Radius || object.radius)) {
      const newRadius = object.Radius || object.radius;
      if (this.radius !== newRadius) {
        this.radius = newRadius;
        this.draw();
        this.nameTag.position.set(0, -this.radius - 10);
      }
    }

    const orientation =
      data.orientation !== undefined ? data.orientation : data.Orientation;
    if (orientation !== undefined && !this.isLocal) {
      this.targetBarrelAngle = orientation;
    }
  }
}
