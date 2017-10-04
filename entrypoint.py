#!/usr/local/bin/python

import os
import re
import sys
import json
import requests


def replace_secrets(env):
    for key, val in env.iteritems():
        if isinstance(val, basestring) and val[0:2] == "${" and val[-1] == "}":
            env[key] = os.getenv(re.sub(r'[\$\{\}]', '', val))
    return env


def main():

    if len(sys.argv) > 2:
        config = json.loads(sys.argv[2]).get("vargs")
        server = config.get("server")
        app = config.get("app_config")
        print "Running legacy version (Drone 0.4)"
    else:
        server = os.getenv("PLUGIN_SERVER")
        app = json.loads(os.getenv("PLUGIN_APP_CONFIG"))

    if not app:
        raise RuntimeError("Missing 'app_config' parameter")
    if not server:
        raise RuntimeError("Missing 'server' parameter")

    print json.dumps(app, indent=4)

    if "env" in app:
        app['env'] = replace_secrets(app["env"])

    resp = requests.put(
        '%s/v2/apps/%s?force=true' % (server, app.get("id")), json=app)

    print resp.text
    resp.raise_for_status()


if __name__ == "__main__":
    main()
