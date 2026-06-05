import { Tank } from "./Tank.js";
import { Bullet } from "./Bullet.js";
import { Food } from "./Food.js";

export class EntityManager {
  constructor(renderer) {
    this.renderer = renderer;
    this.entities = new Map();
  }

  updateEntities(entitiesData) {
    const receivedIDs = new Set();

    for (const data of entitiesData) {
      receivedIDs.add(data.ID);
      let entity = this.entities.get(data.ID);

      if (!entity) {
        entity = this.createEntity(data);
        if (entity) {
          this.entities.set(data.ID, entity);
          this.renderer.entitiesLayer.addChild(entity.container);
        }
      }

      if (entity) {
        entity.updateData(data);
      }
    }

    // Remove entities that are no longer in the snapshot
    for (const [id, entity] of this.entities) {
      if (!receivedIDs.has(id)) {
        entity.destroy();
        this.entities.delete(id);
      }
    }
  }

  createEntity(data) {
    // Determine type based on properties
    if (data.InputVector !== undefined) {
      return new Tank(data.ID);
    } else if (data.OwnerID !== undefined) {
      return new Bullet(data.ID);
    } else if (data.Type !== undefined) {
      return new Food(data.ID);
    }
    return null;
  }

  getEntity(id) {
    return this.entities.get(id);
  }

  update(deltaTime) {
    for (const entity of this.entities.values()) {
      entity.update(deltaTime);
    }
  }
}
