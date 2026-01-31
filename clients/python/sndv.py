import urllib.request
import urllib.error
import json
import time

class SndvError(Exception):
    def __init__(self, code, message):
        self.code = code
        self.message = message
        super().__init__(f"[HTTP {code}] {message}")

class SndvClient:
    def __init__(self, base_url="http://localhost:8080", token=""):
        self.base_url = base_url.rstrip("/")
        self.token = token
        self.headers = {
            "Content-Type": "application/json",
            "Authorization": token
        }

    def _request(self, method, endpoint, payload=None):
        url = f"{self.base_url}{endpoint}"
        data = None
        if payload is not None:
            data = json.dumps(payload).encode('utf-8')

        req = urllib.request.Request(url, data=data, headers=self.headers, method=method)

        try:
            with urllib.request.urlopen(req) as response:
                if response.status == 204:
                    return None
                body = response.read()
                if not body:
                    return None
                return json.loads(body)
        except urllib.error.HTTPError as e:
            raise SndvError(e.code, e.reason)
        except urllib.error.URLError as e:
            raise SndvError(0, f"Connection Failed: {e.reason}")

    def put(self, key, value, ttl=0):
        """
        Write a value to the store.
        :param key: String key
        :param value: String value
        :param ttl: Time to live in seconds (0 = infinite)
        """
        payload = {"key": key, "value": value, "ttl": ttl}
        self._request("POST", "/put", payload)

    def get(self, key):
        """
        Retrieve a value. Returns None if not found or token invalid.
        """
        try:
            resp = self._request("GET", f"/get?key={key}")
            return resp.get("val") if resp else None
        except SndvError as e:
            if e.code == 404:
                return None
            raise e

    def delete(self, key):
        """
        Delete a key (Tombstone).
        """
        self._request("POST", f"/delete?key={key}")

    def metrics(self):
        """
        Fetch server metrics.
        """
        return self._request("GET", "/metrics")

# --- Example Usage ---
if __name__ == "__main__":
    # Example assumes server is running locally
    # You must paste your actual admin token here to run this directly
    TOKEN = "PASTE_YOUR_TOKEN_HERE" 
    
    if TOKEN == "PASTE_YOUR_TOKEN_HERE":
        print("Please edit the file and set TOKEN to run the example.")
    else:
        client = SndvClient("http://localhost:8080", TOKEN)
        
        print("Writing...")
        client.put("demo_key", "Hello World", 60)
        
        print("Reading...")
        val = client.get("demo_key")
        print(f"Got: {val}")
        
        print("Metrics:", client.metrics())