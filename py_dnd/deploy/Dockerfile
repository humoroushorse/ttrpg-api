# Dockerfile
# Uses multi-stage builds requiring Docker 17.05 or higher
# See https://docs.docker.com/develop/develop-images/multistage-build/
ARG PYTHON_DOCKER_IMAGE=python:3.15-alpine

################################################################################
# python-base
#    Creating a python base with shared environment variables
################################################################################
FROM $PYTHON_DOCKER_IMAGE AS python-base

ARG POETRY_VERSION=2.5.1

WORKDIR /tmp

ENV PYTHONUNBUFFERED=1 \
    PYTHONDONTWRITEBYTECODE=1 \
    PIP_NO_CACHE_DIR=off \
    PIP_DISABLE_PIP_VERSION_CHECK=on \
    POETRY_NO_INTERACTION=1

# Poetry
RUN pip install "poetry==${POETRY_VERSION}"
COPY . .

################################################################################
# builder-base
#    Installs the exact versions from poetry.lock into /opt/venv, then adds the
#    app wheel on top. pyproject.toml only sets minimum versions, so letting pip
#    resolve the wheel's dependencies would pull whatever is newest at build time.
################################################################################
FROM python-base AS builder-base

# Poetry installs into an already-active virtualenv instead of creating its own.
# The venv is built at the same path it is copied to, so script shebangs stay valid.
ENV VIRTUAL_ENV=/opt/venv \
    PATH="/opt/venv/bin:$PATH"

# RUN buildDeps="build-essential" \
#     && apt-get update \
#     && apt-get install --no-install-recommends -y \
#         curl \
#         vim \
#     && apt-get install -y --no-install-recommends $buildDeps \
#     && rm -rf /var/lib/apt/lists/*

WORKDIR /tmp

RUN python -m venv "$VIRTUAL_ENV" \
    && poetry sync --only main --no-root \
    && poetry build --format wheel \
    && pip install --no-deps dist/*.whl

################################################################################
# development
#    'development' stage installs all dev deps and can be used to develop code.
#    For example using docker-compose to mount local volume under /app
################################################################################
# FROM python-base as development
FROM $PYTHON_DOCKER_IMAGE AS development

ENV FASTAPI_ENV=production \
    FASTAPI_port=8001 \
    PATH="/opt/venv/bin:$PATH"

WORKDIR /code

COPY --from=builder-base /opt/venv /opt/venv

# Copy alembic config and migrations for database migration support
COPY --from=python-base /tmp/alembic.ini /code/alembic.ini
COPY --from=python-base /tmp/migrations /code/migrations

# poetry run uvicorn --reload --host=0.0.0.0 --port=8001 --app-dir=py_dnd main:app
# CMD ["uvicorn", "--reload", "--host=0.0.0.0", "--port=8001", "py_dnd.main:app"]
WORKDIR /
COPY --from=python-base /tmp/deploy/docker-entrypoint.sh /docker-entrypoint.sh
RUN chmod +x /docker-entrypoint.sh

EXPOSE ${FASTAPI_port}
ENTRYPOINT [ "/docker-entrypoint.sh" ]

################################################################################
# lint (development)
#    'lint' stage runs black and isort
#    running in check mode means build will fail if any linting errors occur
################################################################################

FROM development AS lint
RUN black --config ./pyproject.toml --check app tests
RUN isort --settings-path ./pyproject.toml --recursive --check-only
CMD ["tail", "-f", "/dev/null"]

################################################################################
# test (development)
#    'test' stage runs our unit tests with pytest and
#    coverage.  Build will fail if test coverage is under 95%
################################################################################
FROM development AS test
RUN coverage run --rcfile ./pyproject.toml -m pytest ./tests
RUN coverage report --fail-under 95
