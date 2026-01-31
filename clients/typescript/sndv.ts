export interface SndvConfig {
    baseUrl: string;
    token: string;
}

export interface Metrics {
    WriteOps: number;
    ReadOps: number;
    WALSyncs: number;
    ActiveMemTable: number;
    // Add other metrics as needed
}

export class SndvClient {
    private baseUrl: string;
    private headers: HeadersInit;

    constructor(config: SndvConfig) {
        this.baseUrl = config.baseUrl.replace(/\/$/, "");
        this.headers = {
            "Content-Type": "application/json",
            "Authorization": config.token
        };
    }

    private async request<T>(method: string, endpoint: string, body?: any): Promise<T> {
        const url = `${this.baseUrl}${endpoint}`;
        const options: RequestInit = {
            method,
            headers: this.headers,
            body: body ? JSON.stringify(body) : undefined
        };

        const res = await fetch(url, options);

        if (!res.ok) {
            if (res.status === 404) {
                throw new Error("Key not found");
            }
            throw new Error(`SNDV Error [${res.status}]: ${res.statusText}`);
        }

        const text = await res.text();
        return JSON.parse(text);
    }

    /**
     * Write a value to the store
     * @param key Unique key
     * @param value String value
     * @param ttl Seconds to live (0 = forever)
     */
    async put(key: string, value: string, ttl: number = 0): Promise<void> {
        await this.request("POST", "/put", { key, value, ttl });
    }

    /**
     * Get a value by key
     * @returns The value string or null if missing
     */
    async get(key: string): Promise<string | null> {
        try {
            const res = await this.request<{ val: string }>("GET", `/get?key=${key}`);
            return res.val;
        } catch (e: any) {
            if (e.message === "Key not found") return null;
            throw e;
        }
    }

    /**
     * Delete a key (Create Tombstone)
     */
    async delete(key: string): Promise<void> {
        await this.request("POST", `/delete?key=${key}`);
    }

    /**
     * Get Server Metrics
     */
    async metrics(): Promise<Metrics> {
        return this.request<Metrics>("GET", "/metrics");
    }
}

// --- Usage Example ---
/*
async function main() {
    const client = new SndvClient({
        baseUrl: "http://localhost:8080",
        token: "YOUR_TOKEN_HERE"
    });

    await client.put("ts_key", "Typescript Rules");
    const val = await client.get("ts_key");
    console.log("Value:", val);
}
*/