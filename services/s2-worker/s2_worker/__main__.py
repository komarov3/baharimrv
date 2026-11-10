"""`python -m s2_worker` runs the service.

One process, one worker: the concurrency cap is a thread semaphore, so multiple
uvicorn workers would each get their own copy of it and the cap would quietly
become 2 times N against two public catalogs.
"""

from __future__ import annotations

import os
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
    uvicorn.run(create_app(settings), host="0.0.0.0",
                port=int(os.environ.get("PORT", "9000")), workers=1, log_level="info")
    return 0


if __name__ == "__main__":
    sys.exit(main())
