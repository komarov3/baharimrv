# syntax=docker/dockerfile:1
# BahariMRV s2-worker — Fly.io production image.
#
# The image is dominated by the GDAL and rasterio wheels, so the dependency layer
# is built from pyproject.toml alone and the source is copied after it. Editing a
# module then costs seconds instead of a re-resolve of the geospatial stack.

FROM python:3.12-slim

WORKDIR /app
ENV PYTHONUNBUFFERED=1 \
    PYTHONDONTWRITEBYTECODE=1 \
    PIP_NO_CACHE_DIR=1 \
    PIP_DISABLE_PIP_VERSION_CHECK=1

COPY pyproject.toml ./
RUN pip install --no-cache-dir $(python -c "import tomllib; print(' '.join(tomllib.load(open('pyproject.toml','rb'))['project']['dependencies']))")

COPY s2_worker ./s2_worker

# Tiles go on a mounted volume when TILE_STORE_BACKEND=volume, and nowhere at all
# when the deployment uses the R2 backend.
RUN useradd --uid 10001 --create-home worker \
    && mkdir -p /data/tiles \
    && chown -R worker:worker /app /data
USER worker

ENV PORT=9000 \
    TILE_STORE_ROOT=/data/tiles

EXPOSE 9000

# /v1/health is the one route that needs no token, which is what makes it usable
# as a container health check.
HEALTHCHECK --interval=30s --timeout=5s --start-period=10s --retries=3 \
  CMD python -c "import urllib.request,sys; sys.exit(0 if urllib.request.urlopen('http://127.0.0.1:9000/v1/health', timeout=4).status == 200 else 1)"

CMD ["python", "-m", "s2_worker"]
