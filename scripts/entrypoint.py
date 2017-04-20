#!/usr/local/bin/python

import sys
import json
import requests

if __name__ == "__main__":
    config = json.loads(sys.argv[2]).get("vargs")
    server = config.get("server")
    app_config = config.get("app_config")
    r = requests.put('%s/v2/apps/%s' % (server, app_config.get("id")), json=app_config)
    print r.text