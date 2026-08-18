import random
from locust import HttpUser, task, between

BUYER_USER_ID = "11111111-1111-1111-1111-111111111111"
SELLER_USER_ID = "22222222-2222-2222-2222-222222222222"


class OrderUser(HttpUser):
    wait_time = between(0.1, 0.5)

    # 50-50 split for BUY and SELL
    @task(2)
    def place_buy_order(self):
        self.place_order(BUYER_USER_ID, "BUY")

    @task(2)
    def place_sell_order(self):
        self.place_order(SELLER_USER_ID, "SELL")

    def place_order(self, user_id, side):
        price = f"{random.randint(100, 200)}.00"
        self.client.post(
            "/api/v1/orders", 
            json={
                "user_id": user_id, 
                "ticker": "APPL", 
                "side": side, 
                "qty": f"{random.randint(1,3)}", 
                "price" : price
            }
        )
