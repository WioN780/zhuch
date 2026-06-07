export class Camera {
  constructor(renderer) {
    this.renderer = renderer;
    this.x = 0;
    this.y = 0;
    this.zoom = 1;

    this.targetX = 0;
    this.targetY = 0;
    this.targetZoom = 1;

    this.lerpSpeed = 0.1;
    this.zoomLerpSpeed = 0.05;
  }

  setTarget(pos) {
    this.targetX = pos.x;
    this.targetY = pos.y;
  }

  setZoom(zoom) {
    this.targetZoom = zoom;
  }

  adjustZoom(delta) {
    this.targetZoom = Math.max(0.2, Math.min(2.0, this.targetZoom + delta));
  }

  update(deltaTime) {
    const screenCenterX = window.innerWidth / 2;
    const screenCenterY = window.innerHeight / 2;

    // Smoothly move towards target
    const desiredX = screenCenterX - this.targetX * this.zoom;
    const desiredY = screenCenterY - this.targetY * this.zoom;

    this.x += (desiredX - this.x) * this.lerpSpeed * deltaTime;
    this.y += (desiredY - this.y) * this.lerpSpeed * deltaTime;

    this.zoom += (this.targetZoom - this.zoom) * this.zoomLerpSpeed * deltaTime;
  }
}
