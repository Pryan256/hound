"""
Hound Enrichment Service

Enriches raw transaction data from aggregators:
- Merchant name normalization  (e.g. "SQ *BLUEBOTTLE SF" → "Blue Bottle Coffee")
- Category classification
- Personal finance category mapping

Runs as an internal HTTP service called by the Go API.
Never exposed directly to developers.
"""

import os
import json
import logging
from typing import Optional
from http.server import HTTPServer, BaseHTTPRequestHandler

import anthropic

logging.basicConfig(level=logging.INFO, format="%(asctime)s %(levelname)s %(message)s")
log = logging.getLogger(__name__)

client = anthropic.Anthropic(api_key=os.environ["ANTHROPIC_API_KEY"])

ENRICHMENT_SYSTEM_PROMPT = """You are a financial transaction enrichment engine.
Given a raw transaction description from a bank feed, extract:
1. merchant_name: Clean, human-readable merchant name (e.g. "Blue Bottle Coffee" not "SQ *BLUEBOTTLE SF 12345")
2. category: List of category strings, most specific first (e.g. ["Food and Drink", "Restaurants", "Coffee Shop"])
3. personal_finance_primary: One of: INCOME, TRANSFER_IN, TRANSFER_OUT, LOAN_PAYMENTS, BANK_FEES, ENTERTAINMENT, FOOD_AND_DRINK, GENERAL_MERCHANDISE, HOME_IMPROVEMENT, MEDICAL, PERSONAL_CARE, GENERAL_SERVICES, GOVERNMENT_AND_NON_PROFIT, TRANSPORTATION, TRAVEL, RENT_AND_UTILITIES
4. personal_finance_detailed: More specific subcategory (e.g. "FOOD_AND_DRINK_COFFEE")
5. payment_channel: one of "in store", "online", "other"

Respond ONLY with valid JSON. No explanation. No markdown.
Example: {"merchant_name": "Blue Bottle Coffee", "category": ["Food and Drink", "Coffee"], "personal_finance_primary": "FOOD_AND_DRINK", "personal_finance_detailed": "FOOD_AND_DRINK_COFFEE", "payment_channel": "in store"}"""


def enrich_transaction(raw_name: str, amount: float) -> dict:
    """Enrich a single transaction using Claude."""
    try:
        message = client.messages.create(
            model="claude-haiku-4-5-20251001",  # Haiku for speed + cost efficiency
            max_tokens=256,
            system=ENRICHMENT_SYSTEM_PROMPT,
            messages=[
                {
                    "role": "user",
                    "content": f'Transaction: "{raw_name}", Amount: ${abs(amount):.2f}',
                }
            ],
        )
        return json.loads(message.content[0].text)
    except (json.JSONDecodeError, Exception) as e:
        log.warning(f"enrichment failed for '{raw_name}': {e}")
        return {
            "merchant_name": raw_name,
            "category": ["Uncategorized"],
            "personal_finance_primary": "GENERAL_MERCHANDISE",
            "personal_finance_detailed": "GENERAL_MERCHANDISE_OTHER",
            "payment_channel": "other",
        }


def enrich_batch(transactions: list[dict]) -> list[dict]:
    """Enrich a batch of transactions. Each transaction needs 'name' and 'amount'."""
    results = []
    for txn in transactions:
        enriched = enrich_transaction(txn.get("name", ""), txn.get("amount", 0))
        results.append({**txn, **enriched})
    return results


class EnrichmentHandler(BaseHTTPRequestHandler):
    def log_message(self, format, *args):
        log.info(f"{self.address_string()} {format % args}")

    def do_POST(self):
        if self.path != "/enrich":
            self.send_response(404)
            self.end_headers()
            return

        length = int(self.headers.get("Content-Length", 0))
        body = json.loads(self.rfile.read(length))

        transactions = body.get("transactions", [])
        if not transactions:
            self._json(400, {"error": "transactions array required"})
            return

        enriched = enrich_batch(transactions)
        self._json(200, {"transactions": enriched})

    def _json(self, status: int, data: dict):
        payload = json.dumps(data).encode()
        self.send_response(status)
        self.send_header("Content-Type", "application/json")
        self.send_header("Content-Length", str(len(payload)))
        self.end_headers()
        self.wfile.write(payload)

    def do_GET(self):
        if self.path == "/health":
            self._json(200, {"status": "ok"})
        else:
            self.send_response(404)
            self.end_headers()


if __name__ == "__main__":
    port = int(os.environ.get("PORT", 8081))
    server = HTTPServer(("0.0.0.0", port), EnrichmentHandler)
    log.info(f"enrichment service listening on :{port}")
    server.serve_forever()
