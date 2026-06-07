export class UIManager {
  constructor(game) {
    this.game = game;
    this.container = document.getElementById("ui-layer");
    this.currentScreen = null;
  }

  onStateChange(state) {
    this.clear();
    switch (state) {
      case "MENU":
        this.showMenu();
        break;
      case "CONNECTING":
        this.showConnecting();
        break;
      case "PLAYING":
        this.showHUD();
        break;
      case "DEAD":
        this.showDeathScreen();
        break;
      case "ERROR":
        // Handled separately or as a screen
        break;
    }
  }

  clear() {
    this.container.innerHTML = "";
  }

  showMenu() {
    const menu = document.createElement("div");
    menu.className = "screen menu-screen";
    menu.innerHTML = `
            <div class="menu-background">
                <div class="menu-grid"></div>
            </div>
            <div class="menu-container">
                <div class="menu-content glass">
                    <h1 class="logo">zhuch</h1>
                    <p class="subtitle">----------------------</p>
                    <div class="input-group">
                        <input type="text" id="player-name" placeholder="Enter name..." maxlength="16" value="">
                    </div>
                    <div class="input-group">
                        <select id="room-id">
                            <option value="default">Default Server</option>
                            <option value="local">Local Server (8080)</option>
                            <option value="custom">Custom Server</option>
                        </select>
                    </div>
                    <div class="input-group" id="custom-url-group" style="display: none;">
                        <input type="text" id="custom-url" placeholder="ws://localhost:8080" value="">
                    </div>
                    <div class="menu-buttons">
                        <button id="start-btn" class="button button-primary">Join Game</button>
                        <button id="create-room-btn" class="button">Create Room</button>
                    </div>
                </div>

                <div class="rooms-panel glass">
                    <div class="rooms-header">
                        <h3>Active Rooms</h3>
                        <div class="refresh-indicator" id="refresh-indicator"></div>
                    </div>
                    <div class="rooms-list" id="rooms-list-container">
                        <div class="rooms-placeholder">Fetching rooms...</div>
                    </div>
                </div>
            </div>

                <div id="create-room-modal" class="modal glass wide-modal" style="display: none;">
                    <h3>Create New Room</h3>
                    <div class="modal-scroll-area">
                        <div class="config-columns">
                            <!-- Column 1: Core & World -->
                            <div class="config-column">
                                <h4 class="section-title">Core & World</h4>
                                <div class="input-group">
                                    <label>Room ID</label>
                                    <input type="text" id="new-room-id" placeholder="Room ID..." maxlength="16">
                                </div>
                                <div class="input-row">
                                    <div class="input-group">
                                        <label>Map Width</label>
                                        <input type="number" id="cfg-map-width" value="2000">
                                    </div>
                                    <div class="input-group">
                                        <label>Map Height</label>
                                        <input type="number" id="cfg-map-height" value="2000">
                                    </div>
                                </div>
                                <div class="input-row">
                                    <div class="input-group">
                                        <label>Max Food</label>
                                        <input type="number" id="cfg-max-food" value="50">
                                    </div>
                                    <div class="input-group">
                                        <label>TPS</label>
                                        <input type="number" id="cfg-tps" value="20">
                                    </div>
                                </div>
                                <div class="input-row">
                                    <div class="input-group">
                                        <label>Friction</label>
                                        <input type="number" id="cfg-friction" value="0.9" step="0.05">
                                    </div>
                                    <div class="input-group">
                                        <label>Cell Size</label>
                                        <input type="number" id="cfg-cell-size" value="100">
                                    </div>
                                </div>
                                <div class="input-group">
                                    <label>View Range</label>
                                    <input type="number" id="cfg-view-range" value="800">
                                </div>
                            </div>

                            <!-- Column 2: Tank Physics & Regen -->
                            <div class="config-column">
                                <h4 class="section-title">Tank Physics</h4>
                                <div class="input-row">
                                    <div class="input-group">
                                        <label>Radius</label>
                                        <input type="number" id="cfg-tank-radius" value="20">
                                    </div>
                                    <div class="input-group">
                                        <label>Weight</label>
                                        <input type="number" id="cfg-tank-weight" value="10">
                                    </div>
                                </div>
                                <div class="input-row">
                                    <div class="input-group">
                                        <label>Max Speed</label>
                                        <input type="number" id="cfg-tank-speed" value="15">
                                    </div>
                                    <div class="input-group">
                                        <label>Accel</label>
                                        <input type="number" id="cfg-tank-accel" value="1.5" step="0.1">
                                    </div>
                                </div>
                                <div class="input-row">
                                    <div class="input-group">
                                        <label>Max HP</label>
                                        <input type="number" id="cfg-tank-hp" value="100">
                                    </div>
                                    <div class="input-group">
                                        <label>Body Dmg</label>
                                        <input type="number" id="cfg-tank-dmg" value="20">
                                    </div>
                                </div>

                                <h4 class="section-title">Regeneration</h4>
                                <div class="input-row">
                                    <div class="input-group">
                                        <label>Rate</label>
                                        <input type="number" id="cfg-tank-regen" value="0.02" step="0.01">
                                    </div>
                                    <div class="input-group">
                                        <label>Quick Rate</label>
                                        <input type="number" id="cfg-tank-qregen" value="0.8" step="0.1">
                                    </div>
                                </div>
                                <div class="input-group">
                                    <label>Regen Cooldown (ticks)</label>
                                    <input type="number" id="cfg-tank-regen-cd" value="100">
                                </div>
                            </div>

                            <!-- Column 3: Combat & Effects -->
                            <div class="config-column">
                                <h4 class="section-title">Combat</h4>
                                <div class="input-row">
                                    <div class="input-group">
                                        <label>Muzzle Spd</label>
                                        <input type="number" id="cfg-bullet-speed" value="20">
                                    </div>
                                    <div class="input-group">
                                        <label>Fire CD</label>
                                        <input type="number" id="cfg-fire-cooldown" value="10">
                                    </div>
                                </div>
                                <div class="input-row">
                                    <div class="input-group">
                                        <label>Bullet Dmg</label>
                                        <input type="number" id="cfg-bullet-dmg" value="15">
                                    </div>
                                    <div class="input-group">
                                        <label>Bullet Life</label>
                                        <input type="number" id="cfg-bullet-span" value="60">
                                    </div>
                                </div>
                                <div class="input-row">
                                    <div class="input-group">
                                        <label>Bullet Rad</label>
                                        <input type="number" id="cfg-bullet-radius" value="5">
                                    </div>
                                    <div class="input-group">
                                        <label>Bullet Wgt</label>
                                        <input type="number" id="cfg-bullet-weight" value="1.0" step="0.1">
                                    </div>
                                </div>
                                <div class="input-group">
                                    <label>Recoil Power</label>
                                    <input type="number" id="cfg-recoil" value="2.0" step="0.1">
                                </div>

                                <h4 class="section-title">Food Configs (Square)</h4>
                                <div class="input-row">
                                    <div class="input-group">
                                        <label>HP</label>
                                        <input type="number" id="cfg-food-sq-hp" value="10">
                                    </div>
                                    <div class="input-group">
                                        <label>Score</label>
                                        <input type="number" id="cfg-food-sq-score" value="10">
                                    </div>
                                </div>
                                <h4 class="section-title">Food Configs (Triangle)</h4>
                                <div class="input-row">
                                    <div class="input-group">
                                        <label>HP</label>
                                        <input type="number" id="cfg-food-tr-hp" value="30">
                                    </div>
                                    <div class="input-group">
                                        <label>Score</label>
                                        <input type="number" id="cfg-food-tr-score" value="25">
                                    </div>
                                </div>
                                <h4 class="section-title">Food Configs (Pentagon)</h4>
                                <div class="input-row">
                                    <div class="input-group">
                                        <label>HP</label>
                                        <input type="number" id="cfg-food-pt-hp" value="100">
                                    </div>
                                    <div class="input-group">
                                        <label>Score</label>
                                        <input type="number" id="cfg-food-pt-score" value="100">
                                    </div>
                                </div>
                            </div>
                        </div>
                    </div>
                    <div class="modal-buttons">
                        <button id="confirm-create-btn" class="button button-primary">Create</button>
                        <button id="cancel-create-btn" class="button">Cancel</button>
                    </div>
                </div>
            </div>
        `;
    this.container.appendChild(menu);

    const startBtn = document.getElementById("start-btn");
    const nameInput = document.getElementById("player-name");
    const roomSelect = document.getElementById("room-id");
    const customUrlGroup = document.getElementById("custom-url-group");
    const customUrlInput = document.getElementById("custom-url");

    const createRoomBtn = document.getElementById("create-room-btn");
    const createModal = document.getElementById("create-room-modal");
    const confirmCreateBtn = document.getElementById("confirm-create-btn");
    const cancelCreateBtn = document.getElementById("cancel-create-btn");
    const newRoomIdInput = document.getElementById("new-room-id");

    const roomsListContainer = document.getElementById("rooms-list-container");
    const refreshIndicator = document.getElementById("refresh-indicator");

    // Setup all listeners FIRST
    roomSelect.onchange = () => {
      customUrlGroup.style.display =
        roomSelect.value === "custom" ? "block" : "none";
      updateRoomList();
    };

    customUrlInput.onblur = () => {
      updateRoomList();
    };

    startBtn.onclick = () => {
      const name = nameInput.value.trim();
      if (!name) {
        this.showError("Please enter a name.");
        return;
      }

      const selectedRoom = roomSelect.value;
      let roomID = "default";
      let customURL = null;

      if (selectedRoom === "local") {
        customURL = "localhost:8080";
        roomID = "default";
      } else if (selectedRoom === "custom") {
        customURL = customUrlInput.value.trim();
        roomID = "default";
      } else {
        roomID = selectedRoom;
      }

      this.game.roomController.joinGame(name, roomID, customURL);
    };

    createRoomBtn.onclick = () => {
      console.log("Create room button clicked"); // Debug
      createModal.style.display = "flex";
    };

    cancelCreateBtn.onclick = () => {
      createModal.style.display = "none";
    };

    confirmCreateBtn.onclick = async () => {
      const roomID = newRoomIdInput.value.trim();
      if (!roomID) {
        this.showError("Please enter a room ID.");
        return;
      }

      const config = {
        ticks_per_second: parseInt(document.getElementById("cfg-tps").value),
        friction: parseFloat(document.getElementById("cfg-friction").value),
        map_width: parseFloat(document.getElementById("cfg-map-width").value),
        map_height: parseFloat(document.getElementById("cfg-map-height").value),
        cell_size: parseFloat(document.getElementById("cfg-cell-size").value),
        max_food: parseInt(document.getElementById("cfg-max-food").value),

        tank_radius: parseFloat(
          document.getElementById("cfg-tank-radius").value,
        ),
        tank_max_health: parseFloat(
          document.getElementById("cfg-tank-hp").value,
        ),
        tank_body_damage: parseFloat(
          document.getElementById("cfg-tank-dmg").value,
        ),
        tank_weight: parseFloat(
          document.getElementById("cfg-tank-weight").value,
        ),
        tank_max_speed: parseFloat(
          document.getElementById("cfg-tank-speed").value,
        ),
        tank_acceleration: parseFloat(
          document.getElementById("cfg-tank-accel").value,
        ),
        view_range: parseFloat(document.getElementById("cfg-view-range").value),

        tank_regen_rate: parseFloat(
          document.getElementById("cfg-tank-regen").value,
        ),
        tank_quick_regen_rate: parseFloat(
          document.getElementById("cfg-tank-qregen").value,
        ),
        tank_regen_cooldown: parseInt(
          document.getElementById("cfg-tank-regen-cd").value,
        ),
        tank_fire_cooldown: parseInt(
          document.getElementById("cfg-fire-cooldown").value,
        ),

        bullet_muzzle_speed: parseFloat(
          document.getElementById("cfg-bullet-speed").value,
        ),
        bullet_weight: parseFloat(
          document.getElementById("cfg-bullet-weight").value,
        ),
        bullet_radius: parseFloat(
          document.getElementById("cfg-bullet-radius").value,
        ),
        bullet_damage: parseFloat(
          document.getElementById("cfg-bullet-dmg").value,
        ),
        bullet_lifespan: parseInt(
          document.getElementById("cfg-bullet-span").value,
        ),
        recoil_power: parseFloat(document.getElementById("cfg-recoil").value),

        food_configs: {
          square: {
            health: parseFloat(document.getElementById("cfg-food-sq-hp").value),
            score_value: parseFloat(
              document.getElementById("cfg-food-sq-score").value,
            ),
            body_damage: 5,
            weight: 0.5,
            size: 15,
          },
          triangle: {
            health: parseFloat(document.getElementById("cfg-food-tr-hp").value),
            score_value: parseFloat(
              document.getElementById("cfg-food-tr-score").value,
            ),
            body_damage: 10,
            weight: 0.8,
            size: 12,
          },
          pentagon: {
            health: parseFloat(document.getElementById("cfg-food-pt-hp").value),
            score_value: parseFloat(
              document.getElementById("cfg-food-pt-score").value,
            ),
            body_damage: 20,
            weight: 2.0,
            size: 20,
          },
        },
      };

      const roomSelect = document.getElementById("room-id");
      let customURL = null;
      if (roomSelect.value === "local") customURL = "localhost:8080";
      else if (roomSelect.value === "custom")
        customURL = customUrlInput.value.trim();

      try {
        const success = await this.game.roomController.createRoom(
          roomID,
          config,
          customURL,
        );
        if (success) {
          createModal.style.display = "none";
          // Update room list immediately
          updateRoomList();
          roomSelect.value = roomID;
          this.showError("Room created successfully!");
        }
      } catch (err) {
        this.showError(`Failed to create room: ${err.message}`);
      }
    };

    const updateRoomList = async () => {
      try {
        refreshIndicator.classList.add("refreshing");
        const selectedRoomValue = roomSelect.value;
        const customURL =
          selectedRoomValue === "custom"
            ? customUrlInput.value.trim()
            : selectedRoomValue === "local"
              ? "localhost:8080"
              : null;

        const rooms = await this.game.roomController.fetchRooms(customURL);

        // Preserve current selection if it still exists, or default to first
        const currentVal = roomSelect.value;

        // Clear all except hardcoded options in select
        while (roomSelect.options.length > 3) {
          roomSelect.remove(3);
        }

        // Clear panel
        roomsListContainer.innerHTML = "";

        if (!rooms || rooms.length === 0) {
          roomsListContainer.innerHTML =
            '<div class="rooms-placeholder">No active rooms found</div>';
        } else {
          rooms.forEach((room) => {
            // Dropdown populating
            if (room.ID !== "default") {
              const option = document.createElement("option");
              option.value = room.ID;
              option.text = room.ID;
              roomSelect.add(option);
            }

            // List panel populating
            const item = document.createElement("div");
            item.className = "room-item glass";
            const tps = room.Game?.Config?.ticks_per_second || "??";
            const players = room.Hub?.Clients
              ? Object.keys(room.Hub.Clients).length
              : 0;

            item.innerHTML = `
                <div class="room-info">
                    <div class="room-name">${room.ID}</div>
                    <div class="room-details">${tps} TPS • ${players} players</div>
                </div>
                <button class="join-room-small-btn button">Join</button>
            `;

            item.querySelector(".join-room-small-btn").onclick = () => {
              const name = nameInput.value.trim();
              if (!name) {
                this.showError("Please enter a name first.");
                nameInput.focus();
                return;
              }
              this.game.roomController.joinGame(name, room.ID, customURL);
            };

            roomsListContainer.appendChild(item);
          });
        }

        // Try to restore selection
        roomSelect.value = currentVal;
      } catch (err) {
        console.error("Failed to update room list:", err);
      } finally {
        setTimeout(() => refreshIndicator.classList.remove("refreshing"), 500);
      }
    };

    // Now start the async loop
    updateRoomList();
    setInterval(updateRoomList, 10000);
  }

  showConnecting() {
    const screen = document.createElement("div");
    screen.className = "screen connecting-screen";
    screen.innerHTML = `
            <div class="loader-content">
                <div class="spinner"></div>
                <p>Connecting to server...</p>
            </div>
        `;
    this.container.appendChild(screen);
  }

  showDeathScreen() {
    const screen = document.createElement("div");
    screen.className = "screen death-screen glass";
    screen.innerHTML = `
            <div class="death-content">
                <h2 class="death-title">YOU DIED</h2>
                <p>Better luck next time!</p>
                <div class="death-buttons">
                    <button id="respawn-btn" class="button button-primary">Respawn</button>
                    <button id="menu-btn" class="button">Main Menu</button>
                </div>
            </div>
        `;
    this.container.appendChild(screen);

    document.getElementById("respawn-btn").onclick = () => {
      this.game.respawn();
    };

    document.getElementById("menu-btn").onclick = () => {
      this.game.setState("MENU");
    };
  }

  showHUD() {
    const hud = document.createElement("div");
    hud.className = "hud";
    hud.innerHTML = `
            <div class="hud-top-left">
                <div class="debug-panel glass">
                    <div class="debug-item" id="debug-coords">0, 0</div>
                    <div class="debug-item" id="debug-metrics">TPS: 0 | Entities: 0</div>
                    <div class="debug-item" id="debug-ping">Ping: 0ms</div>
                </div>
            </div>
            <div class="hud-top-right">
                <div class="leaderboard glass" id="leaderboard">
                    <h3>Leaderboard</h3>
                    <div id="leaderboard-list"></div>
                </div>
            </div>
            <div class="hud-bottom-center">
                <div class="stats-bar glass">
                    <div class="stat"><span class="label">SCORE</span> <span id="stat-score">0</span></div>
                    <div class="stat"><span class="label">KILLS</span> <span id="stat-kills">0</span></div>
                </div>
            </div>
        `;
    this.container.appendChild(hud);
  }

  showError(message) {
    const error = document.createElement("div");
    error.className = "error-toast glass";

    // If we're in menu or connecting, it's likely a non-fatal validation error
    const isFatal = this.game.state === "ERROR";

    error.innerHTML = `
      <p>${message}</p>
      ${isFatal ? '<button onclick="window.location.reload()" class="button">Reload</button>' : ""}
    `;
    this.container.appendChild(error);

    if (!isFatal) {
      setTimeout(() => {
        error.style.opacity = "0";
        setTimeout(() => error.remove(), 500);
      }, 3000);
    }
  }

  updateHUD(entities, metrics) {
    // Update score, kills
    const playerTank = entities.find(
      (e) => (e.id || e.ID) === this.game.renderer.playerID,
    );
    if (playerTank) {
      const score =
        playerTank.score !== undefined ? playerTank.score : playerTank.Score;
      const kills =
        playerTank.kills !== undefined ? playerTank.kills : playerTank.Kills;
      document.getElementById("stat-score").innerText = Math.floor(score || 0);
      document.getElementById("stat-kills").innerText = kills || 0;
    }

    // Leaderboard
    const leaderboardList = document.getElementById("leaderboard-list");
    if (leaderboardList) {
      const tanks = entities
        .filter((e) => e.score !== undefined || e.Score !== undefined)
        .sort((a, b) => {
          const sA = a.score !== undefined ? a.score : a.Score;
          const sB = b.score !== undefined ? b.score : b.Score;
          return (sB || 0) - (sA || 0);
        })
        .slice(0, 5);

      leaderboardList.innerHTML = tanks
        .map((t, i) => {
          const id = t.id || t.ID;
          const score = t.score !== undefined ? t.score : t.Score;
          const name = t.name || t.Name || "Tank";
          return `
          <div class="leaderboard-item ${id === this.game.renderer.playerID ? "self" : ""}">
            <span>${i + 1}. ${name}</span>
            <span>${Math.floor(score || 0)}</span>
          </div>
        `;
        })
        .join("");
    }
  }
}
