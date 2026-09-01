import requests
import json
import sys

# This script assumes you have the `goulm-memory-serve` binary running on port 8080.
# To start the server: go run ./cmd/serve -addr :8080 -dir ./memory-vault

BASE_URL = "http://localhost:8080/api/v1"

def check_server():
    try:
        resp = requests.get(f"{BASE_URL}/healthz")
        if resp.status_code != 200:
            print("Server is not healthy.")
            sys.exit(1)
        print("✅ Server is running!")
    except requests.exceptions.ConnectionError:
        print("❌ Could not connect to server. Did you start `cmd/serve`?")
        sys.exit(1)

def remember_fact():
    payload = {
        "key": "python-agent-fact",
        "category": "fact",
        "content": "The Python agent successfully connected to the Go server using the REST API.",
        "tags": ["python", "integration", "rest"]
    }
    resp = requests.post(f"{BASE_URL}/remember", json=payload)
    if resp.status_code == 200:
        print("✅ Fact remembered successfully.")
    else:
        print(f"❌ Failed to remember: {resp.text}")

def recall_knowledge():
    payload = {
        "query": "How did the python agent connect?",
        "limit": 5
    }
    resp = requests.post(f"{BASE_URL}/recall", json=payload)
    if resp.status_code == 200:
        results = resp.json().get("results", [])
        print(f"✅ Recalled {len(results)} items:")
        for r in results:
            print(f"  -> [Score: {r['score']:.2f}] {r['capsule']['content']}")
    else:
        print(f"❌ Failed to recall: {resp.text}")

if __name__ == "__main__":
    check_server()
    remember_fact()
    recall_knowledge()
