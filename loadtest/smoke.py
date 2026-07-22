import json
import urllib.request

BASE_URL = "http://localhost:8080"
BUYER_USER_ID = "11111111-1111-1111-1111-111111111111"
SELLER_USER_ID = "22222222-2222-2222-2222-222222222222"

def post_order(user_id: str, side:str, quantity:str, price:str) -> dict:
    """ POST /api/v1/orders and return parsed JSON body """
    request_body = json.dumps({
        "user_id" : user_id,
        "ticker": "APPL", 
        "side": side,
        "qty": quantity,
        "price": price
    }).encode()

    http_request = urllib.request.Request(
        BASE_URL + "/api/v1/orders", 
        data=request_body,
        headers={"Content-Type": "application/json"},
        method="POST",
    )

    with urllib.request.urlopen(http_request) as http_response:
        return json.loads(http_response.read().decode())

def main():
    # Order matters for a clean demo: rest liquidity (SELL), then take it (BUY).
    sell_response = post_order(SELLER_USER_ID, "SELL", "10", "150.00")
    print("SELL:", sell_response)
    buy_response = post_order(BUYER_USER_ID, "BUY", "10", "150.00")
    print("BUY:", buy_response)
    print("Check SQL: SELECT * FROM trades; SELECT id, balance FROM users;")



if __name__ == "__main__":
    main()