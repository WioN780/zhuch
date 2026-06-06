import { Tank } from "./Tank.js";
import { Bullet } from "./Bullet.js";
import { Food } from "./Food.js";

export class EntityManager {
  constructor(renderer) {
    this.renderer = renderer;
    this.entities = new Map();
    this.pool = {
      tank: [],
      bullet: [],
      food: [],
    };
  }

  updateEntities(entitiesData) {
    const receivedIDs = new Set();

    for (const data of entitiesData) {
      const id = data.id || data.ID;
      receivedIDs.add(id);
      let entity = this.entities.get(id);

      if (!entity) {
        entity = this.getOrCreateEntity(data);
        if (entity) {
          this.entities.set(id, entity);
          this.renderer.entitiesLayer.addChild(entity.container);
        }
      }

      if (entity) {
        entity.updateData(data);
      }
    }

    // Return entities that are no longer in the snapshot to the pool
    for (const [id, entity] of this.entities) {
      if (!receivedIDs.has(id)) {
        this.recycleEntity(entity);
        this.entities.delete(id);
      }
    }
  }

  getOrCreateEntity(data) {
    const type = this.getEntityType(data);
    let entity = this.pool[type]?.pop();

    if (!entity) {
      entity = this.createEntity(type, data);
    } else {
      entity.id = data.id || data.ID;
      entity.reset?.();
    }
    return entity;
  }

  getEntityType(data) {
    if (data.orientation !== undefined || data.Orientation !== undefined)
      return "tank";
    if (data.owner_id !== undefined || data.OwnerID !== undefined)
      return "bullet";
    return "food";
  }

  createEntity(type, data) {
    const id = data.id || data.ID;
    if (type === "tank") return new Tank(id, this);
    if (type === "bullet") return new Bullet(id, this);
    if (type === "food") return new Food(id, this);
    return null;
  }

  recycleEntity(entity) {
    entity.container.parent?.removeChild(entity.container);
    let type = "food";
    if (entity instanceof Tank) type = "tank";
    else if (entity instanceof Bullet) type = "bullet";

    this.pool[type].push(entity);
  }

  getEntity(id) {
    return this.entities.get(id);
  }

  update(deltaTime, deltaMS) {
    for (const entity of this.entities.values()) {
      entity.update(deltaTime, deltaMS);
    }
  }
}
