import { Graphics } from "pixi.js";
import { EntityBase } from "./EntityBase.js";

export class Tank extends EntityBase {
  constructor(id, manager) {
    super(id, manager);

    this.body = new Graphics();
    this.barrel = new Graphics();

    this.container.addChild(this.barrel);
    this.container.addChild(this.body);

    this.radius = 20;
    this.barrelAngle = 0;
    this.targetBarrelAngle = 0;

    this.draw();
  }

  draw() {
    // Body circle
    this.body.clear();
    this.body.circle(0, 0, this.radius);
    this.body.fill({ color: 0x1a1a1a });
    this.body.stroke({ color: 0xffffff, width: 1.5, alpha: 0.8 });

    // Barrel rectangle
    this.barrel.clear();
    this.barrel.rect(
      0,
      -this.radius * 0.4,
      this.radius * 1.5,
      this.radius * 0.8,
    );
    this.barrel.fill({ color: 0x1a1a1a });
    this.barrel.stroke({ color: 0xffffff, width: 1.5, alpha: 0.8 });
  }

  updateData(data) {
    super.updateData(data);
    const object = data.object || data.Object;
    if (object && (object.Radius || object.radius)) {
      const newRadius = object.Radius || object.radius;
      if (this.radius !== newRadius) {
        this.radius = newRadius;
        this.draw();
      }
    }

    const orientation = data.orientation !== undefined ? data.orientation : data.Orientation;
    if (orientation !== undefined) {
      // Only update target if it's not the local player
      if (this.id !== this.manager?.renderer?.playerID) {
        this.targetBarrelAngle = orientation;
      }
    }
  }

  update(deltaTime) {
    super.update(deltaTime);

    // Shortest path interpolation for angles
    let diff = this.targetBarrelAngle - this.barrelAngle;
    while (diff > Math.PI) diff -= Math.PI * 2;
    while (diff < -Math.PI) diff += Math.PI * 2;

    this.barrelAngle += diff * 0.3 * deltaTime;
    this.barrel.rotation = this.barrelAngle;
  }
}
