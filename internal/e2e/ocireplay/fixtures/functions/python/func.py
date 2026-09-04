import io
import json


def handler(ctx, data: io.BytesIO | None = None):
    del ctx
    payload = data.getvalue().decode("utf-8") if data else ""
    return json.dumps({"message": "osok-replay", "input": payload})
