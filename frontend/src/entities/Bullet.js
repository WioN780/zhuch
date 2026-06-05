import { Graphics } from "pixi.js";
import { EntityBase } from "./EntityBase.js";

export class Bullet extends EntityBase {
  constructor(id) {
    super(id);
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
    if (
      data.Object &&
      data.Object.Radius &&
      this.radius !== data.Object.Radius
    ) {
      this.radius = data.Object.Radius;
      this.draw();
    }
  }
}
