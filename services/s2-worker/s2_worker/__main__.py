"""`python -m s2_worker` runs the service.

One process, one worker: the concurrency cap is a thread semaphore, so multiple
uvicorn workers would each get their own copy of it and the cap would quietly
become 2 times N against two public catalogs.
"""

from __future__ import annotations

import os
import socket
import sys

from .app import create_app
from .config import Settings


def main() -> int:
    import uvicorn

    settings = Settings.from_env()
    if not settings.token:
        print("S2_WORKER_TOKEN is unset; every request would be refused. Refusing to start.",
              file=sys.stderr)
        return 1
    host = os.environ.get("HOST", "0.0.0.0")
    port = int(os.environ.get("PORT", "9000"))
    family = socket.AF_INET6 if ":" in host else socket.AF_INET
    # Fly's health probes use IPv4; direct .internal requests use IPv6.
    with socket.create_server((host, port), family=family, dualstack_ipv6=host == "::") as listener:
        server = uvicorn.Server(uvicorn.Config(create_app(settings), workers=1, log_level="info"))
        server.run(sockets=[listener])
    return 0


if __name__ == "__main__":
    sys.exit(main())
