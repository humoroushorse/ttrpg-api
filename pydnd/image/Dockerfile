# Dockerfile
# Uses multi-stage builds requiring Docker 17.05 or higher
# See https://docs.docker.com/develop/develop-images/multistage-build/

################################################################################
# python-base
#    Creating a python base with shared environment variables
################################################################################
FROM python:3.12.2-slim AS python-base

ENV PYTHONUNBUFFERED=1 \
    PYTHONDONTWRITEBYTECODE=1 \
    PIP_NO_CACHE_DIR=off \
    PIP_DISABLE_PIP_VERSION_CHECK=on \
    PIP_DEFAULT_TIMEOUT=100 \
    POETRY_HOME="/opt/poetry" \
    POETRY_VIRTUALENVS_IN_PROJECT=true \
    POETRY_NO_INTERACTION=1 \
    PYSETUP_PATH="/opt/pysetup" \
    VENV_PATH="/opt/pysetup/.venv"

ENV PATH="$POETRY_HOME/bin:$VENV_PATH/bin:$PATH"

################################################################################
# builder-base
#    'builder-base' stage is used to build dependencies
#    python3-dev, libpq-dev -> required for psycopg2
################################################################################
FROM python-base AS builder-base
RUN buildDeps="build-essential" \
    && apt-get update \
    && apt-get install --no-install-recommends -y \
        curl \
        vim \
        python3-dev \
        libpq-dev \
        gcc \
    && apt-get install -y --no-install-recommends $buildDeps \
    && rm -rf /var/lib/apt/lists/*

# Install Poetry - respects $POETRY_VERSION & $POETRY_HOME
ENV POETRY_VERSION=1.7.1
SHELL ["/bin/bash", "-o", "pipefail", "-c"]
RUN curl -sSL https://install.python-poetry.org | POETRY_HOME=${POETRY_HOME} python3 - --version ${POETRY_VERSION} && \
    chmod a+x /opt/poetry/bin/poetry

# We copy our Python requirements here to cache them and install only runtime deps.
# using poetry --no-root in poetry install fixes the "/opt/pysetup/app/dnd does not contain any element" error
WORKDIR $PYSETUP_PATH
COPY ./poetry.lock ./pyproject.toml ./
RUN poetry install --only main --no-root # respects

################################################################################
# development
#    'development' stage installs all dev deps and can be used to develop code.
#    For example using docker-compose to mount local volume under /app
################################################################################
FROM python-base as development
ENV FASTAPI_ENV=production

# Copying poetry and venv into image
COPY --from=builder-base $POETRY_HOME $POETRY_HOME
COPY --from=builder-base $PYSETUP_PATH $PYSETUP_PATH

# Copying in our entrypoint
COPY docker-entrypoint.sh /docker-entrypoint.sh
RUN chmod +x /docker-entrypoint.sh

# venv already has runtime deps installed we get a quicker install
WORKDIR $PYSETUP_PATH
RUN poetry install --no-root

WORKDIR /app
COPY . .

EXPOSE 8001
ENTRYPOINT /docker-entrypoint.sh $0 $@

# ENV PYTHONPATH /app
# poetry run uvicorn --host=127.0.0.1 --port=8001 dnd.main:app
CMD ["uvicorn", "--reload", "--host=0.0.0.0", "--port=8001", "--app-dir=app", "dnd.main:app"]

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
#    'test' stage runs our unit tests with pytest and coverage.
#    Build will fail if test coverage is under 95%
################################################################################
FROM development AS test
RUN coverage run --rcfile ./pyproject.toml -m pytest ./tests
RUN coverage report --fail-under 95

################################################################################
# production
#    'production' stage uses the clean 'python-base' stage and copyies
#    in only our runtime deps that were installed in the 'builder-base'
# ! NOTICE: deprecated
# !   this is currently gunicorn handling multiple processes in say one pod
# !   with kubernetes we just essentially use uvicorn in one pod and let kubernetes scale it
# !      by spawning more processes
################################################################################
FROM python-base AS production
ENV FASTAPI_ENV=production

# as this is deprecated, we just remove it from our poetry in our app
#    so we have to reinstall here
COPY --from=builder-base $POETRY_HOME $POETRY_HOME
COPY --from=builder-base $PYSETUP_PATH $PYSETUP_PATH
WORKDIR $PYSETUP_PATH
RUN poetry add gunicorn

COPY --from=builder-base $VENV_PATH $VENV_PATH
COPY gunicorn_conf.py /gunicorn_conf.py

COPY docker-entrypoint.sh /docker-entrypoint.sh
RUN chmod +x /docker-entrypoint.sh

# Create user with the name poetry
RUN groupadd -g 1500 poetry && \
    useradd -m -u 1500 -g poetry poetry

COPY --chown=poetry:poetry ./app /app
USER poetry
WORKDIR /app

ENTRYPOINT /docker-entrypoint.sh $0 $@
# TODO: install unicorn in poetry
CMD ["gunicorn", "--worker-class uvicorn.workers.UvicornWorker", "--config /gunicorn_conf.py", "--pythonpath '/app'", "dnd.main:app"]
