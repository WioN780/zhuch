export class RoomController {
    constructor(game) {
        this.game = game;
    }

    /**
     * Get the base URL based on the user's selection
     */
    getBaseURL(customURL = null) {
        let url = customURL || "zhuch-production.up.railway.app";
        if (!url.startsWith("http")) {
            url = url.includes("localhost") ? `http://${url}` : `https://${url}`;
        }
        return url;
    }

    /**
     * API: Fetch all active rooms from a server
     */
    async fetchRooms(customURL = null) {
        const baseURL = this.getBaseURL(customURL);
        try {
            const response = await fetch(`${baseURL}/rooms`);
            if (!response.ok) throw new Error("Failed to fetch rooms");
            return await response.json();
        } catch (err) {
            console.error("RoomController Error:", err);
            return [];
        }
    }

    /**
     * API: Create a new room
     * mode: "ffa" | "zombies" | "boss" | "practice" (optional, server defaults to "ffa")
     */
    async createRoom(roomID, config, customURL = null, mode = null, botModel = null, botCount = null) {
        const baseURL = this.getBaseURL(customURL);
        const body = { id: roomID, config };
        if (mode) body.mode = mode;
        if (botModel) body.bot_model = botModel;
        if (botCount) body.bot_count = botCount;
        const response = await fetch(`${baseURL}/create`, {
            method: "POST",
            headers: { "Content-Type": "application/json" },
            body: JSON.stringify(body)
        });

        if (!response.ok) {
            const msg = await response.text();
            throw new Error(msg);
        }
        return true;
    }

    /**
     * API: Delete a room (built-in rooms are protected server-side)
     */
    async deleteRoom(roomID, customURL = null) {
        const baseURL = this.getBaseURL(customURL);
        const response = await fetch(`${baseURL}/delete`, {
            method: "POST",
            headers: { "Content-Type": "application/json" },
            body: JSON.stringify({ id: roomID })
        });
        if (!response.ok) {
            const msg = await response.text();
            throw new Error(msg);
        }
        return true;
    }

    /**
     * Action: Join a game
     */
    async joinGame(playerName, roomID, customURL = null) {
        try {
            this.game.setState("CONNECTING");
            await this.game.socket.connect(playerName, roomID, customURL);
            this.game.setState("PLAYING");
        } catch (err) {
            console.error("Join failed:", err);
            this.game.setState("ERROR");
            this.game.ui.showError(`Connection failed: ${err.message}`);
        }
    }
}
