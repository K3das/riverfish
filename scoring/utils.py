from datetime import datetime
import requests
import math

# token = "[REDACTED]"
# discord_auth = {"Authorization": "Bot " + token}
# channel_baseurl = "https://discord.com/api/v9/channels/[REDACTED]/messages"
channel_baseurl = "https://discord.com/api/webhooks/[REDACTED]"
discord_auth = {}

embeds = []

def send_report(domain, score, ml_score):
    global embeds
    embeds.append({
        "type":
        "rich",
        "title":
        "",
        "description":
        f"`{domain}`",
        "color":
        0xFFCCCB,
        "fields": [{
            "name": "`score` (higher is sussier)",
            "value": f"`{score:.{2}f}`",
            "inline": True
        }, {
            "name": "`ml_score` (lower is sussier)",
            "value": f"`{ml_score:.{4}f}`",
            "inline": True
        }],
        "timestamp":
        datetime.utcnow().isoformat(),
        "footer": {
            "text": "yarn.network"
        }
    })
    if len(embeds) != 3:
        return
    send_result = requests.post(channel_baseurl,
                                json={
                                    "username": "susdomain",
                                    "embeds": embeds
                                },
                                headers=discord_auth)
    embeds = []
    if not 200 <= send_result.status_code < 300:
        print(f"Report message not sent ({send_result.status_code})")
        return
    if "token" not in discord_auth:
        return
    message_id = send_result.json()['id']
    crosspost_result = requests.post(
        f"{channel_baseurl}/{message_id}/crosspost", headers=discord_auth)
    if not 200 <= crosspost_result.status_code < 300:
        print(
            f"Report message not crossposted ({crosspost_result.status_code})")


def entropy(string):
    """Calculates the Shannon entropy of a string"""
    prob = [
        float(string.count(c)) / len(string)
        for c in dict.fromkeys(list(string))
    ]
    entropy = -sum([p * math.log(p) / math.log(2.0) for p in prob])
    return entropy