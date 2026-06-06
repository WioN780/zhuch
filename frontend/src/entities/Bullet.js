import { Graphics } from "pixi.js";
import { EntityBase } from "./EntityBase.js";

export class Bullet extends EntityBase {
  constructor(id, manager) {
    super(id, manager);
    this.graphics = new Graphics();
    this.container.addChild(this.graphics);
    this.radius = 5;
    this.draw();
  }

  draw() {
    this.graphics.clear();
    this.graphics.circle(0, 0, this.radius);
    this.graphics.fill({ color: 0xffffff, alpha: 0.9 });
  }

  updateData(data) {
    super.updateData(data);

    // Always keep authoritative state updated for smooth extrapolation
    const object = data.object || data.Object;
    const center = object?.Center || object?.center || { X: 0, Y: 0 };
    const vel = data.vel || data.Vel;

    this.position = { x: center.X || center.x, y: center.Y || center.y };
    this.velocity = { x: vel?.X || vel?.x || 0, y: vel?.Y || vel?.y || 0 };

    if (object && (object.Radius || object.radius)) {
      const newRadius = object.Radius || object.radius;
      if (this.radius !== newRadius) {
        this.radius = newRadius;
        this.draw();
      }
    }
  }

  update(deltaTime, deltaMS) {
    if (!this.initialized) return;

    // Simple extrapolation for 60fps smoothness
    // Move at server-provided velocity between updates
    const ratio = deltaMS / 50;
    this.position.x += this.velocity.x * ratio;
    this.position.y += this.velocity.y * ratio;

    this.syncContainer();
  }
}
